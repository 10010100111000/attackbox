package labnetwork

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"strings"
	"testing"
	"time"
)

type rewrite struct{ base string }

func (r rewrite) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	u := *req.URL
	clone.URL = &u
	clone.URL.Scheme = "http"
	clone.URL.Host = strings.TrimPrefix(r.base, "http://")
	return http.DefaultTransport.RoundTrip(clone)
}

func TestDockerInternalAuxReservationAndOwnership(t *testing.T) {
	n, err := Plan(netip.MustParsePrefix("10.201.0.0/16"), spec("one"), nil)
	if err != nil {
		t.Fatal(err)
	}
	var stored dockerNetwork
	creates, deletes := 0, 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			if stored.ID == "" {
				w.WriteHeader(404)
				return
			}
			json.NewEncoder(w).Encode(stored)
		case "POST":
			if err := json.NewDecoder(r.Body).Decode(&stored); err != nil {
				t.Error(err)
			}
			stored.ID = "immutable-docker-id"
			creates++
			w.WriteHeader(201)
			json.NewEncoder(w).Encode(map[string]string{"Id": stored.ID})
		case "DELETE":
			if r.URL.Path != "/v1.47/networks/immutable-docker-id" {
				t.Error("deleted by mutable name")
			}
			deletes++
			w.WriteHeader(204)
		}
	}))
	defer srv.Close()
	d := &localDocker{client: &http.Client{Transport: rewrite{srv.URL}}}
	for i := 0; i < 2; i++ {
		id, err := d.Ensure(context.Background(), n, "node-a")
		if err != nil {
			t.Fatal(err)
		}
		n.DockerID = id
	}
	if creates != 1 || !stored.Internal || stored.EnableIPv6 || stored.IPAM.Config[0].AuxiliaryAddresses["attackbox-sandbox"] != "10.201.0.14" {
		t.Fatal(stored)
	}
	stored.Labels[labelOwner] = "foreign"
	if err := d.Remove(context.Background(), n, "node-a"); err == nil {
		t.Fatal("foreign network removed")
	}
	stored.Labels[labelOwner] = n.Spec.OwnerID
	stored.Containers = map[string]json.RawMessage{"container": json.RawMessage(`{}`)}
	if err := d.Remove(context.Background(), n, "node-a"); err == nil {
		t.Fatal("active network removed")
	}
	stored.Containers = nil
	stored.IPAM.Config[0].AuxiliaryAddresses = nil
	if err := d.Verify(context.Background(), n, "node-a"); err == nil {
		t.Fatal("lost reservation accepted")
	}
	stored.IPAM.Config[0].AuxiliaryAddresses = map[string]string{"attackbox-sandbox": n.SandboxIP.String()}
	if err := d.Remove(context.Background(), n, "node-a"); err != nil {
		t.Fatal(err)
	}
	if deletes != 1 {
		t.Fatal(deletes)
	}
}

// This smoke test requires only the explicitly selected local Docker daemon.
// It proves Docker's actual network configuration, not host/namespace traffic.
func TestDockerNetworkRealEngine(t *testing.T) {
	socket := os.Getenv("ATTACKBOX_TEST_DOCKER_SOCKET")
	if socket == "" {
		t.Skip("set ATTACKBOX_TEST_DOCKER_SOCKET to opt into local Docker")
	}
	d, err := newLocalDocker(socket)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	all, err := d.Networks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	used, err := occupiedDocker(all)
	if err != nil {
		t.Fatal(err)
	}
	suffix := shortHash(time.Now().String(), 12)
	s := spec("test_" + suffix)
	s.OwnerID = "fixture-" + suffix
	n, err := Plan(netip.MustParsePrefix("172.29.240.0/24"), s, used)
	if err != nil {
		t.Fatal(err)
	}
	id, err := d.Ensure(ctx, n, "fixture-"+suffix)
	if err != nil {
		t.Fatal(err)
	}
	n.DockerID = id
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := d.Remove(ctx, n, "fixture-"+suffix); err != nil {
			t.Error(err)
		}
	})
	if err := d.Verify(ctx, n, "fixture-"+suffix); err != nil {
		t.Fatal(err)
	}
	if id2, err := d.Ensure(ctx, n, "fixture-"+suffix); err != nil || id2 != id {
		t.Fatal(id2, err)
	}
}
