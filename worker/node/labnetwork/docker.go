package labnetwork

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"path/filepath"
	"time"
)

type dockerIPAM struct {
	Driver string
	Config []struct {
		Subnet             string
		Gateway            string
		IPRange            string
		AuxiliaryAddresses map[string]string
	}
}

type dockerNetwork struct {
	ID         string `json:"Id"`
	Name       string
	Driver     string
	Internal   bool
	EnableIPv6 bool
	Options    map[string]string
	Labels     map[string]string
	IPAM       dockerIPAM
	Containers map[string]json.RawMessage
}

type localDocker struct {
	client    *http.Client
	transport *http.Transport
}

func newLocalDocker(socket string) (*localDocker, error) {
	if !filepath.IsAbs(socket) {
		return nil, fmt.Errorf("Docker requires an absolute local Unix socket")
	}
	t := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socket)
	}}
	return &localDocker{client: &http.Client{Transport: t, Timeout: 10 * time.Second}, transport: t}, nil
}

func (d *localDocker) Close() {
	if d.transport != nil {
		d.transport.CloseIdleConnections()
	}
}

type dockerError int

func (e dockerError) Error() string { return fmt.Sprintf("Docker network API HTTP %d", int(e)) }

func (d *localDocker) request(ctx context.Context, method, path string, payload, result any) error {
	var body bytes.Buffer
	if payload != nil {
		if err := json.NewEncoder(&body).Encode(payload); err != nil {
			return err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, "http://docker/v1.47"+path, &body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("local Docker unavailable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return dockerError(resp.StatusCode)
	}
	if result != nil {
		return json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(result)
	}
	return nil
}

func (d *localDocker) Networks(ctx context.Context) ([]dockerNetwork, error) {
	var out []dockerNetwork
	err := d.request(ctx, "GET", "/networks", nil, &out)
	return out, err
}

func networkOptions(n Network) map[string]string {
	return map[string]string{
		"com.docker.network.bridge.name":                 n.Bridge,
		"com.docker.network.bridge.enable_ip_masquerade": "false",
		"com.docker.network.bridge.enable_icc":           "true",
	}
}

func networkLabels(n Network, node string) map[string]string {
	return map[string]string{labelNode: node, labelKey: n.ID, labelOwner: n.Spec.OwnerID}
}

func checkNetwork(actual dockerNetwork, n Network, node string) error {
	if actual.ID == "" || n.DockerID != "" && actual.ID != n.DockerID || actual.Name != n.Name || actual.Driver != "bridge" || !actual.Internal || actual.EnableIPv6 {
		return fmt.Errorf("Docker lab network identity or driver mismatch")
	}
	for k, v := range networkLabels(n, node) {
		if actual.Labels[k] != v {
			return fmt.Errorf("Docker network ownership mismatch")
		}
	}
	for k, v := range networkOptions(n) {
		if actual.Options[k] != v {
			return fmt.Errorf("Docker network option mismatch: %s", k)
		}
	}
	if actual.IPAM.Driver != "default" || len(actual.IPAM.Config) != 1 {
		return fmt.Errorf("Docker IPAM mismatch")
	}
	p := actual.IPAM.Config[0]
	if p.Subnet != n.Subnet.String() || p.Gateway != n.Gateway.String() || p.IPRange != "" || len(p.AuxiliaryAddresses) != 1 || p.AuxiliaryAddresses["attackbox-sandbox"] != n.SandboxIP.String() {
		return fmt.Errorf("Docker IPAM reservation mismatch")
	}
	return nil
}

func (d *localDocker) inspect(ctx context.Context, name string) (dockerNetwork, error) {
	var n dockerNetwork
	err := d.request(ctx, "GET", "/networks/"+url.PathEscape(name), nil, &n)
	return n, err
}

func (d *localDocker) Ensure(ctx context.Context, n Network, node string) (string, error) {
	actual, err := d.inspect(ctx, n.Name)
	if err == nil {
		return actual.ID, checkNetwork(actual, n, node)
	}
	if !errors.Is(err, dockerError(404)) {
		return "", err
	}
	// A missing recorded network is not adopted or recreated with a new Docker ID.
	if n.DockerID != "" {
		return "", fmt.Errorf("recorded Docker network is missing")
	}
	if _, err := net.InterfaceByName(n.Bridge); err == nil {
		return "", fmt.Errorf("bridge name already exists")
	}
	actual = dockerNetwork{Name: n.Name, Driver: "bridge", Internal: true, Options: networkOptions(n), Labels: networkLabels(n, node), IPAM: dockerIPAM{Driver: "default"}}
	actual.IPAM.Config = append(actual.IPAM.Config, struct {
		Subnet             string
		Gateway            string
		IPRange            string
		AuxiliaryAddresses map[string]string
	}{Subnet: n.Subnet.String(), Gateway: n.Gateway.String(), AuxiliaryAddresses: map[string]string{"attackbox-sandbox": n.SandboxIP.String()}})
	var created struct {
		ID string `json:"Id"`
	}
	if err := d.request(ctx, "POST", "/networks/create", actual, &created); err != nil {
		return "", err
	}
	actual, err = d.inspect(ctx, n.Name)
	if err != nil {
		return "", err
	}
	if err := checkNetwork(actual, n, node); err != nil {
		return "", err
	}
	if created.ID != "" && created.ID != actual.ID {
		return "", fmt.Errorf("Docker network changed during creation")
	}
	return actual.ID, nil
}

func (d *localDocker) Verify(ctx context.Context, n Network, node string) error {
	actual, err := d.inspect(ctx, n.Name)
	if err != nil {
		return err
	}
	return checkNetwork(actual, n, node)
}

func (d *localDocker) Remove(ctx context.Context, n Network, node string) error {
	actual, err := d.inspect(ctx, n.Name)
	if errors.Is(err, dockerError(404)) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := checkNetwork(actual, n, node); err != nil {
		return err
	}
	if len(actual.Containers) != 0 {
		return fmt.Errorf("lab network still has Docker endpoints")
	}
	// Delete by immutable Docker ID after inspection, never by a replaceable name.
	err = d.request(ctx, "DELETE", "/networks/"+url.PathEscape(actual.ID), nil, nil)
	if errors.Is(err, dockerError(404)) {
		return nil
	}
	return err
}

func occupiedDocker(networks []dockerNetwork) ([]netip.Prefix, error) {
	var out []netip.Prefix
	for _, n := range networks {
		for _, c := range n.IPAM.Config {
			if c.Subnet == "" {
				continue
			}
			p, err := netip.ParsePrefix(c.Subnet)
			if err != nil {
				return nil, fmt.Errorf("invalid Docker network subnet")
			}
			out = append(out, p)
		}
	}
	return out, nil
}
