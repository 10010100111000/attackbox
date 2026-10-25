package node

import (
	"attackbox/protocol"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const targetContainerLabel = "io.attackbox.target"
const targetOwnerLabel = "io.attackbox.owner"
const targetNodeLabel = "io.attackbox.node"
const targetConfigLabel = "io.attackbox.config"

var targetIDPattern = regexp.MustCompile(`^tgt_[a-f0-9]{16,64}$`)
var targetOwnerPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
var targetImagePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/@-]{0,254}$`)

type targetRecord struct {
	Cleaned     bool                   `json:"cleaned,omitempty"`
	NodeID      string                 `json:"node_id"`
	Request     protocol.TargetRequest `json:"request"`
	Fingerprint string                 `json:"fingerprint"`
	Desired     string                 `json:"desired"`
}

type targetContainer struct {
	ID     string `json:"Id"`
	Config struct {
		Labels map[string]string
		Image  string
	}
	State struct {
		Status  string
		Running bool
		Health  *struct{ Status string }
	}
	NetworkSettings struct {
		Networks map[string]struct{ IPAddress string }
	}
}

// DockerTargetManager persists lifecycle intent independently from disposable
// Btrfs sandboxes. Neither disconnect nor process shutdown destroys containers.
// Operations are serialized in this small single-node implementation.
type DockerTargetManager struct {
	clock            func() time.Time
	imageMu          sync.Mutex
	imageJobs        map[string]string
	mu               sync.Mutex
	cfg              DockerTargetNetworkConfig
	nodeID, stateDir string
	client           *http.Client
	records          map[string]targetRecord
}

func NewDockerTargetManager(cfg DockerTargetNetworkConfig, nodeID, stateDir string) (*DockerTargetManager, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("docker_targets_disabled")
	}
	if nodeID == "" {
		return nil, fmt.Errorf("target_node_id_required")
	}
	if err := os.MkdirAll(stateDir, 0700); err != nil {
		return nil, fmt.Errorf("target_state_unavailable")
	}
	m := &DockerTargetManager{cfg: cfg, nodeID: nodeID, stateDir: stateDir, records: map[string]targetRecord{}}
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", cfg.SocketPath)
	}}
	m.client = &http.Client{Transport: transport, Timeout: 125 * time.Second}
	files, err := os.ReadDir(stateDir)
	if err != nil {
		return nil, fmt.Errorf("target_state_unavailable")
	}
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(stateDir, file.Name()))
		if err != nil {
			return nil, fmt.Errorf("target_state_corrupt")
		}
		var r targetRecord
		if json.Unmarshal(raw, &r) != nil || validateTargetRequest(r.Request, r.Desired != "stopped" || r.Request.Image != "") != nil || file.Name() != r.Request.TargetID+".json" || (r.Desired != "running" && r.Desired != "stopped") || r.NodeID != nodeID || r.Fingerprint != targetFingerprint(r.Request) {
			return nil, fmt.Errorf("target_state_corrupt")
		}
		m.records[r.Request.TargetID] = r
	}
	return m, nil
}

func validateTargetRequest(r protocol.TargetRequest, start bool) error {
	if !targetIDPattern.MatchString(r.TargetID) || !targetOwnerPattern.MatchString(r.OwnerID) {
		return fmt.Errorf("invalid_target_identity")
	}
	if !start {
		return nil
	}
	if !targetImagePattern.MatchString(r.Image) || (!strings.Contains(r.Image, "@sha256:") && !strings.Contains(filepath.Base(r.Image), ":")) {
		return fmt.Errorf("explicit_image_tag_required")
	}
	if strings.Contains(r.Image, "@") {
		parts := strings.Split(r.Image, "@sha256:")
		if len(parts) != 2 || len(parts[1]) != 64 {
			return fmt.Errorf("invalid_image_digest")
		}
		if _, err := hex.DecodeString(parts[1]); err != nil {
			return fmt.Errorf("invalid_image_digest")
		}
	}
	if len(r.Ports) > 16 {
		return fmt.Errorf("too_many_target_ports")
	}
	seen := map[string]bool{}
	for _, p := range r.Ports {
		key := fmt.Sprintf("%d/%s", p.Port, p.Protocol)
		if p.Port < 1 || p.Port > 65535 || (p.Protocol != "tcp" && p.Protocol != "udp") || seen[key] {
			return fmt.Errorf("invalid_target_port")
		}
		seen[key] = true
	}
	return nil
}

func targetFingerprint(r protocol.TargetRequest) string {
	raw, _ := json.Marshal(struct {
		Owner, Image string
		Ports        []protocol.TargetPort
	}{r.OwnerID, r.Image, r.Ports})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func (m *DockerTargetManager) save(r targetRecord) error {
	raw, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("target_state_write_failed")
	}
	f, err := os.CreateTemp(m.stateDir, ".target-*")
	if err != nil {
		return fmt.Errorf("target_state_write_failed")
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(raw); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(name, filepath.Join(m.stateDir, r.Request.TargetID+".json"))
	}
	if err != nil {
		return fmt.Errorf("target_state_write_failed")
	}
	// Rename has committed the intent in this process. Sync the directory so a
	// power loss cannot forget the tombstone after we acknowledge deletion.
	m.records[r.Request.TargetID] = r
	dir, err := os.Open(m.stateDir)
	if err != nil {
		return fmt.Errorf("target_state_write_failed")
	}
	syncErr := dir.Sync()
	dir.Close()
	if syncErr != nil {
		return fmt.Errorf("target_state_write_failed")
	}
	return nil
}

func (m *DockerTargetManager) labels(r targetRecord) map[string]string {
	return map[string]string{targetContainerLabel: r.Request.TargetID, targetOwnerLabel: r.Request.OwnerID, targetNodeLabel: m.nodeID, targetConfigLabel: r.Fingerprint}
}

func (m *DockerTargetManager) owned(c targetContainer, r targetRecord) bool {
	for k, v := range m.labels(r) {
		if c.Config.Labels[k] != v {
			return false
		}
	}
	return true
}

func (m *DockerTargetManager) inspect(ctx context.Context, id string) (targetContainer, int, error) {
	var c targetContainer
	code, err := dockerNetworkRequest(ctx, m.client, "GET", "/containers/"+url.PathEscape(id)+"/json", nil, &c)
	if err != nil && code != 404 {
		return c, code, fmt.Errorf("docker_inspect_failed")
	}
	return c, code, err
}

func (m *DockerTargetManager) ensureImage(ctx context.Context, image string) error {
	code, err := dockerNetworkRequest(ctx, m.client, "GET", "/images/"+url.PathEscape(image)+"/json", nil, nil)
	if err == nil {
		return nil
	}
	if code != 404 {
		return fmt.Errorf("image_inspect_failed")
	}
	req, err := http.NewRequestWithContext(ctx, "POST", "http://docker/v1.47/images/create?fromImage="+url.QueryEscape(image), nil)
	if err != nil {
		return fmt.Errorf("image_pull_failed")
	}
	downloadClient := *m.client
	downloadClient.Timeout = 30 * time.Minute
	resp, err := downloadClient.Do(req)
	if err != nil {
		return fmt.Errorf("image_pull_failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("image_pull_failed")
	}
	dec := json.NewDecoder(io.LimitReader(resp.Body, 32<<20))
	for {
		var status struct {
			Error string `json:"error"`
		}
		err := dec.Decode(&status)
		if err == io.EOF {
			break
		}
		if err != nil || status.Error != "" {
			return fmt.Errorf("image_pull_failed")
		}
	}
	// A truncated or empty progress stream does not prove the image exists.
	if _, err := dockerNetworkRequest(ctx, m.client, "GET", "/images/"+url.PathEscape(image)+"/json", nil, nil); err != nil {
		return fmt.Errorf("image_pull_failed")
	}
	return nil
}

func (m *DockerTargetManager) Start(ctx context.Context, req protocol.TargetRequest) (protocol.TargetSummary, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	req.Ports = append([]protocol.TargetPort(nil), req.Ports...)
	if ctx.Err() != nil {
		return protocol.TargetSummary{}, fmt.Errorf("target_operation_cancelled")
	}
	if err := validateTargetRequest(req, true); err != nil {
		return protocol.TargetSummary{}, err
	}
	r, exists := m.records[req.TargetID]
	if exists && r.Request.OwnerID != req.OwnerID {
		return protocol.TargetSummary{}, fmt.Errorf("target_owner_mismatch")
	}
	if exists && (r.Desired == "stopped" || m.expired(r)) {
		s, err := m.stopRecord(ctx, r)
		if err != nil {
			return s, err
		}
		return s, fmt.Errorf("target_lease_ended")
	}
	if req.ExpiresAt.IsZero() {
		return protocol.TargetSummary{}, fmt.Errorf("target_expiry_required")
	}
	if !req.ExpiresAt.After(m.now()) {
		return m.stopRecord(ctx, targetRecord{NodeID: m.nodeID, Request: req, Fingerprint: targetFingerprint(req), Desired: "stopped"})
	}
	ctx, cancel := context.WithDeadline(ctx, req.ExpiresAt)
	defer cancel()
	if exists && !r.Request.ExpiresAt.Equal(req.ExpiresAt) {
		return protocol.TargetSummary{}, fmt.Errorf("target_lease_conflict")
	}
	if exists {
		if r.Fingerprint != targetFingerprint(req) {
			return protocol.TargetSummary{}, fmt.Errorf("target_configuration_conflict")
		}
		if r.Desired == "stopped" {
			return protocol.TargetSummary{}, fmt.Errorf("target_already_stopped")
		}
	} else {
		active := 0
		for _, record := range m.records {
			if record.Desired == "running" {
				active++
			}
		}
		if active >= 16 {
			return protocol.TargetSummary{}, fmt.Errorf("target_capacity_exceeded")
		}
		r = targetRecord{NodeID: m.nodeID, Request: req, Fingerprint: targetFingerprint(req), Desired: "running"}
		r.Request.BootID = ""
		if err := m.save(r); err != nil {
			return protocol.TargetSummary{}, err
		}
	}
	c, code, err := m.inspect(ctx, "attackbox-"+req.TargetID)
	if err != nil && code != 404 {
		return protocol.TargetSummary{}, err
	}
	if code == 404 {
		// Validate network ownership/options at the point of use, not only at boot.
		var network dockerNetwork
		if _, err := dockerNetworkRequest(ctx, m.client, "GET", "/networks/"+url.PathEscape(m.cfg.Name), nil, &network); err != nil {
			return protocol.TargetSummary{}, fmt.Errorf("target_network_unavailable")
		}
		if err := checkTargetNetwork(network, m.cfg); err != nil {
			return protocol.TargetSummary{}, fmt.Errorf("target_network_conflict")
		}
		if err := m.requireImage(ctx, req.Image); err != nil {
			return protocol.TargetSummary{}, err
		}
		body := map[string]any{
			"Image": req.Image, "Labels": m.labels(r),
			"HostConfig":       map[string]any{"NetworkMode": m.cfg.Name, "Memory": int64(512 << 20), "MemorySwap": int64(512 << 20), "NanoCpus": int64(1e9), "PidsLimit": 256, "Privileged": false, "SecurityOpt": []string{"no-new-privileges:true"}, "RestartPolicy": map[string]string{"Name": "no"}, "LogConfig": map[string]any{"Type": "json-file", "Config": map[string]string{"max-size": "10m", "max-file": "2"}}},
			"NetworkingConfig": map[string]any{"EndpointsConfig": map[string]any{m.cfg.Name: map[string]any{}}},
		}
		var created struct {
			ID string `json:"Id"`
		}
		if _, err := dockerNetworkRequest(ctx, m.client, "POST", "/containers/create?name="+url.QueryEscape("attackbox-"+req.TargetID), body, &created); err != nil {
			return protocol.TargetSummary{}, fmt.Errorf("target_create_failed")
		}
		c, _, err = m.inspect(ctx, created.ID)
		if err != nil {
			return protocol.TargetSummary{}, fmt.Errorf("target_create_unconfirmed")
		}
	}
	if !m.owned(c, r) {
		return protocol.TargetSummary{}, fmt.Errorf("target_container_not_owned")
	}
	if !c.State.Running {
		if _, err := dockerNetworkRequest(ctx, m.client, "POST", "/containers/"+url.PathEscape(c.ID)+"/start", nil, nil); err != nil {
			return protocol.TargetSummary{}, fmt.Errorf("target_start_failed")
		}
		c, _, err = m.inspect(ctx, c.ID)
		if err != nil {
			return protocol.TargetSummary{}, fmt.Errorf("target_start_unconfirmed")
		}
	}
	return m.summary(ctx, r, c), nil
}

func (m *DockerTargetManager) Stop(ctx context.Context, req protocol.TargetRequest) (protocol.TargetSummary, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := validateTargetRequest(req, false); err != nil {
		return protocol.TargetSummary{}, err
	}
	r, exists := m.records[req.TargetID]
	if !exists {
		// A stop may arrive before a lost/retried start. Persist the tombstone even
		// when no container exists, so the delayed start cannot create one later.
		identity := protocol.TargetRequest{TargetID: req.TargetID, OwnerID: req.OwnerID}
		r = targetRecord{NodeID: m.nodeID, Request: identity, Fingerprint: targetFingerprint(identity), Desired: "stopped"}
	}
	if r.Request.OwnerID != req.OwnerID {
		return protocol.TargetSummary{}, fmt.Errorf("target_owner_mismatch")
	}
	// Durable tombstone is written before Docker deletion, blocking late starts
	// even after a Worker restart. A new lifecycle always needs a new TargetID.
	return m.stopRecord(ctx, r)
}

func (m *DockerTargetManager) stopRecord(ctx context.Context, r targetRecord) (protocol.TargetSummary, error) {
	if r.Desired != "stopped" || !r.Cleaned {
		r.Desired = "stopped"
		if err := m.save(r); err != nil {
			return protocol.TargetSummary{}, err
		}
	}
	s, err := m.remove(ctx, r)
	if err == nil && !r.Cleaned {
		r.Cleaned = true
		if e := m.save(r); e != nil {
			return s, e
		}
	}
	return s, err
}

func (m *DockerTargetManager) remove(ctx context.Context, r targetRecord) (protocol.TargetSummary, error) {
	s := protocol.TargetSummary{ExpiresAt: r.Request.ExpiresAt, TargetID: r.Request.TargetID, OwnerID: r.Request.OwnerID, State: "cleanup_pending", Readiness: "not_running"}
	c, code, err := m.inspect(ctx, "attackbox-"+r.Request.TargetID)
	if code == 404 {
		s.State = "stopped"
		return s, nil
	}
	if err != nil {
		return s, fmt.Errorf("target_cleanup_unconfirmed")
	}
	if !m.owned(c, r) {
		return s, fmt.Errorf("target_container_not_owned")
	}
	// Force removal also works for an unresponsive guest. v=true removes anonymous
	// image volumes; it never removes shared named volumes (none are configured).
	if _, err := dockerNetworkRequest(ctx, m.client, "DELETE", "/containers/"+url.PathEscape(c.ID)+"?force=true&v=true", nil, nil); err != nil {
		return s, fmt.Errorf("target_cleanup_unconfirmed")
	}
	if _, code, _ := m.inspect(ctx, "attackbox-"+r.Request.TargetID); code != 404 {
		return s, fmt.Errorf("target_cleanup_unconfirmed")
	}
	s.State = "stopped"
	return s, nil
}

func (m *DockerTargetManager) Inspect(ctx context.Context, req protocol.TargetRequest) (protocol.TargetSummary, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := validateTargetRequest(req, false); err != nil {
		return protocol.TargetSummary{}, err
	}
	r, ok := m.records[req.TargetID]
	if !ok {
		return protocol.TargetSummary{}, fmt.Errorf("target_not_found")
	}
	if r.Request.OwnerID != req.OwnerID {
		return protocol.TargetSummary{}, fmt.Errorf("target_owner_mismatch")
	}
	if r.Desired == "stopped" || m.expired(r) {
		return m.stopRecord(ctx, r)
	}
	c, code, err := m.inspect(ctx, "attackbox-"+req.TargetID)
	if code == 404 {
		return protocol.TargetSummary{TargetID: req.TargetID, OwnerID: req.OwnerID, ExpiresAt: r.Request.ExpiresAt, State: "missing", Readiness: "not_running"}, nil
	}
	if err != nil {
		return protocol.TargetSummary{}, err
	}
	if !m.owned(c, r) {
		return protocol.TargetSummary{}, fmt.Errorf("target_container_not_owned")
	}
	return m.summary(ctx, r, c), nil
}

func (m *DockerTargetManager) summary(ctx context.Context, r targetRecord, c targetContainer) protocol.TargetSummary {
	s := protocol.TargetSummary{ExpiresAt: r.Request.ExpiresAt, TargetID: r.Request.TargetID, OwnerID: r.Request.OwnerID, ContainerID: c.ID, State: c.State.Status, Readiness: "not_running"}
	if !c.State.Running || c.State.Status != "running" {
		return s
	}
	endpoint, ok := c.NetworkSettings.Networks[m.cfg.Name]
	ip, err := netip.ParseAddr(endpoint.IPAddress)
	subnet, _ := netip.ParsePrefix(m.cfg.SubnetCIDR)
	if !ok || err != nil || !subnet.Contains(ip) {
		s.Readiness = "network_unavailable"
		return s
	}
	s.IP = ip.String()
	if c.State.Health != nil {
		s.Ready = c.State.Health.Status == "healthy"
		s.Readiness = c.State.Health.Status
		return s
	}
	s.Readiness = "unverified"
	checked := false
	for _, p := range r.Request.Ports {
		if p.Protocol != "tcp" {
			continue
		}
		checked = true
		probeCtx, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
		conn, err := (&net.Dialer{}).DialContext(probeCtx, "tcp", net.JoinHostPort(s.IP, strconv.Itoa(p.Port)))
		cancel()
		if err != nil {
			s.Readiness = "waiting_for_ports"
			return s
		}
		conn.Close()
	}
	if checked {
		s.Ready = true
		s.Readiness = "ports_open"
	}
	return s
}

// Snapshot never waits for a long image pull. Its container list comes from
// Docker, not the Btrfs sandbox inventory. Orphaned owned containers are reported
// and are never silently destroyed on Control/Worker reconnect.
func (m *DockerTargetManager) Inventory(ctx context.Context) protocol.TargetInventory {
	result := protocol.TargetInventory{Instances: []protocol.TargetSummary{}}
	filters, _ := json.Marshal(map[string][]string{"label": {targetNodeLabel + "=" + m.nodeID, targetContainerLabel}})
	var containers []struct {
		ID              string `json:"Id"`
		Labels          map[string]string
		State           string
		NetworkSettings struct {
			Networks map[string]struct{ IPAddress string }
		}
	}
	if _, err := dockerNetworkRequest(ctx, m.client, "GET", "/containers/json?all=true&filters="+url.QueryEscape(string(filters)), nil, &containers); err != nil {
		result.Error = "docker_inventory_unavailable"
		return result
	}
	// TryLock keeps heartbeat latency bounded while start/stop is in flight.
	if !m.mu.TryLock() {
		result.Error = "target_operation_in_progress"
		return result
	}
	defer m.mu.Unlock()
	seen := map[string]bool{}
	for _, c := range containers {
		id := c.Labels[targetContainerLabel]
		if c.Labels[targetNodeLabel] != m.nodeID || !targetIDPattern.MatchString(id) {
			continue
		}
		seen[id] = true
		r, ok := m.records[id]
		if !ok {
			result.Instances = append(result.Instances, protocol.TargetSummary{TargetID: id, OwnerID: c.Labels[targetOwnerLabel], ContainerID: c.ID, State: "orphaned", Readiness: "unverified"})
			continue
		}
		if !m.owned(targetContainer{Config: struct {
			Labels map[string]string
			Image  string
		}{Labels: c.Labels}}, r) {
			result.Error = "target_inventory_conflict"
			return result
		}
		if r.Desired == "stopped" || m.expired(r) {
			s, err := m.stopRecord(ctx, r)
			if err != nil {
				s.State = "cleanup_pending"
			}
			result.Instances = append(result.Instances, s)
			continue
		}
		inspected, _, err := m.inspect(ctx, c.ID)
		if err != nil {
			result.Error = "docker_inventory_unavailable"
			return result
		}
		result.Instances = append(result.Instances, m.summary(ctx, r, inspected))
	}
	for id, r := range m.records {
		if !seen[id] && r.Desired == "running" {
			result.Instances = append(result.Instances, protocol.TargetSummary{TargetID: id, OwnerID: r.Request.OwnerID, ExpiresAt: r.Request.ExpiresAt, State: "missing", Readiness: "not_running"})
		}
	}
	result.Available = true
	return result
}

// Keep durable Docker intent outside the Btrfs instances tree: that tree is
// intentionally cleaned by the existing restart-as-reset lifecycle.
func targetStateDirectory(cfg *NodeConfig) string {
	if cfg.Network.DockerTargets.StateDir != "" {
		return cfg.Network.DockerTargets.StateDir
	}
	instances := cfg.Runtime.InstancesDir
	if instances == "" {
		instances = "/var/lib/attackbox/instances"
	}
	return filepath.Join(filepath.Dir(filepath.Clean(instances)), "docker-target-state")
}

func newConfiguredTargetManager(cfg *NodeConfig) (*DockerTargetManager, error) {
	instances := cfg.Runtime.InstancesDir
	if instances == "" {
		instances = "/var/lib/attackbox/instances"
	}
	instancesAbs, err := filepath.Abs(instances)
	if err != nil {
		return nil, fmt.Errorf("invalid_target_state_directory")
	}
	stateAbs, err := filepath.Abs(targetStateDirectory(cfg))
	if err != nil {
		return nil, fmt.Errorf("invalid_target_state_directory")
	}
	// Resolve existing parent symlinks as well as direct lexical nesting.
	resolve := func(p string) string {
		if real, e := filepath.EvalSymlinks(p); e == nil {
			return real
		}
		if real, e := filepath.EvalSymlinks(filepath.Dir(p)); e == nil {
			return filepath.Join(real, filepath.Base(p))
		}
		return p
	}
	instancesAbs = resolve(instancesAbs)
	stateAbs = resolve(stateAbs)
	rel, err := filepath.Rel(instancesAbs, stateAbs)
	if err != nil || rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
		return nil, fmt.Errorf("target_state_must_be_outside_btrfs_instances")
	}
	return NewDockerTargetManager(cfg.Network.DockerTargets, cfg.Node.NodeID, stateAbs)
}

// Close releases HTTP connections only. Running targets deliberately survive.
func (m *DockerTargetManager) Close() { m.client.CloseIdleConnections() }
