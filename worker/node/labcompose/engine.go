package labcompose

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// CLIConfig is trusted node configuration, never filled from a student's API.
type CLIConfig struct{ DockerBinary, DockerSocket, DockerConfig string }
type CLI struct {
	cfg        CLIConfig
	httpClient *http.Client
}

func NewCLI(cfg CLIConfig) (*CLI, error) {
	if cfg.DockerBinary == "" {
		path, err := exec.LookPath("docker")
		if err != nil {
			return nil, err
		}
		cfg.DockerBinary = path
	}
	if !filepath.IsAbs(cfg.DockerBinary) || !filepath.IsAbs(cfg.DockerSocket) {
		return nil, fmt.Errorf("Docker binary and Unix socket must be absolute")
	}
	if cfg.DockerConfig == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		cfg.DockerConfig = filepath.Join(home, ".docker")
	}
	if !filepath.IsAbs(cfg.DockerConfig) {
		return nil, fmt.Errorf("Docker config must be absolute")
	}
	socketPath := cfg.DockerSocket
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", socketPath)
		},
		MaxIdleConns:        16,
		MaxIdleConnsPerHost: 16,
		IdleConnTimeout:     90 * time.Second,
	}
	return &CLI{
		cfg:        cfg,
		httpClient: &http.Client{Transport: transport},
	}, nil
}

type limitedOutput struct {
	bytes.Buffer
	overflow bool
}

func (b *limitedOutput) Write(p []byte) (int, error) {
	n := len(p)
	left := (4 << 20) - b.Len()
	if left < len(p) {
		b.overflow = true
		if left > 0 {
			b.Buffer.Write(p[:left])
		}
	} else {
		b.Buffer.Write(p)
	}
	return n, nil
}

func (c *CLI) run(ctx context.Context, args ...string) ([]byte, error) {
	limit := 2 * time.Minute
	if len(args) >= 2 && (args[0] == "image" && args[1] == "pull" || args[0] == "compose" && slices.Contains(args, "build")) {
		limit = 30 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, limit)
	defer cancel()
	cmd := exec.CommandContext(ctx, c.cfg.DockerBinary, append([]string{"--host=unix://" + c.cfg.DockerSocket}, args...)...)
	configureDockerCommand(cmd)
	// No inherited application secrets, COMPOSE_*, DOCKER_HOST or dotenv files.
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "DOCKER_CONFIG=" + c.cfg.DockerConfig, "COMPOSE_DISABLE_ENV_FILE=1"}
	var out limitedOutput
	cmd.Stdout = &out
	cmd.Stderr = &limitedOutput{}
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("Docker operation failed")
	}
	if out.overflow {
		return nil, fmt.Errorf("Docker response too large")
	}
	return out.Bytes(), nil
}
func (c *CLI) Check(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	b, err := c.run(ctx, "compose", "version", "--short")
	if err != nil {
		return err
	}
	if err := checkComposeVersion(string(b)); err != nil {
		return err
	}
	log.Printf("[COMPOSE] CLI version %s (tested baseline %s)", strings.TrimSpace(string(b)), ComposeVersion)
	b, err = c.run(ctx, "version", "--format", "{{.Server.Version}}")
	if err != nil {
		return err
	}
	v := strings.Split(strings.TrimSpace(string(b)), ".")
	if len(v) < 2 {
		return fmt.Errorf("unknown Docker version")
	}
	major, e1 := strconv.Atoi(v[0])
	minor, e2 := strconv.Atoi(v[1])
	if e1 != nil || e2 != nil || major < 27 || major == 27 && minor < 5 {
		return fmt.Errorf("Docker Engine 27.5+ required")
	}
	return nil
}
func (c *CLI) image(ctx context.Context, ref string) (string, error) {
	b, err := c.run(ctx, "image", "inspect", "--format", "{{.Id}}", ref)
	if err != nil {
		return "", err
	}
	id := strings.TrimSpace(string(b))
	if !imageIDPattern.MatchString(id) {
		return "", fmt.Errorf("invalid Docker image ID")
	}
	return id, nil
}
func (c *CLI) PrepareImage(ctx context.Context, ref string) (string, error) {
	if ref == "" || strings.HasPrefix(ref, "-") {
		return "", fmt.Errorf("invalid image reference")
	}
	if id, err := c.image(ctx, ref); err == nil {
		return id, nil
	}
	return c.RefreshImage(ctx, ref)
}

// RefreshImage intentionally pulls even when the tag already exists locally.
func (c *CLI) RefreshImage(ctx context.Context, ref string) (string, error) {
	if ref == "" || strings.HasPrefix(ref, "-") {
		return "", fmt.Errorf("invalid image reference")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	if _, err := c.run(ctx, "image", "pull", ref); err != nil {
		return "", err
	}
	return c.image(ctx, ref)
}

func (c *CLI) ImagePresent(ctx context.Context, id string) error {
	if !imageIDPattern.MatchString(id) {
		return fmt.Errorf("prepared image ID required")
	}
	actual, err := c.image(ctx, id)
	if err != nil {
		return err
	}
	if actual != id {
		return fmt.Errorf("prepared image changed")
	}
	return nil
}

func ownedLabels(node string, r Record) map[string]any {
	return map[string]any{labelPrefix + "node": node, labelPrefix + "owner": r.Request.OwnerID, labelPrefix + "instance": r.Request.ID, labelPrefix + "deployment": r.Request.DeploymentID}
}
func checkLabels(labels map[string]string, r Record) error {
	for key, value := range map[string]string{labelPrefix + "node": r.NodeID, labelPrefix + "owner": r.Request.OwnerID, labelPrefix + "instance": r.Request.ID, labelPrefix + "deployment": r.Request.DeploymentID} {
		if labels[key] != value {
			return fmt.Errorf("project resource ownership mismatch")
		}
	}
	if labels[labelPrefix+"node"] == "" || labels["com.docker.compose.project"] != r.Project {
		return fmt.Errorf("project resource ownership mismatch")
	}
	return nil
}

type inspectedContainer struct {
	ID     string `json:"Id"`
	Config struct{ Labels map[string]string }
	State  struct {
		Status   string
		ExitCode int
		Health   *struct{ Status string }
	}
	NetworkSettings struct {
		Networks map[string]struct{ IPAddress string }
	}
}

func (c *CLI) dockerGet(ctx context.Context, pathAndQuery string, out any) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://docker"+pathAndQuery, nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("Docker operation failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<20))
		return fmt.Errorf("Docker operation failed")
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, (4<<20)+1))
	if err != nil {
		return fmt.Errorf("Docker operation failed")
	}
	if len(body) > 4<<20 {
		return fmt.Errorf("Docker response too large")
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("invalid container inventory")
	}
	return nil
}

func (c *CLI) inventorySocket(ctx context.Context, r Record) ([]inspectedContainer, error) {
	filterJSON, err := json.Marshal(map[string][]string{
		"label": {"com.docker.compose.project=" + r.Project},
	})
	if err != nil {
		return nil, err
	}
	var listed []struct {
		ID string `json:"Id"`
	}
	query := "/containers/json?all=1&filters=" + url.QueryEscape(string(filterJSON))
	if err := c.dockerGet(ctx, query, &listed); err != nil {
		return nil, err
	}
	if len(listed) == 0 {
		return nil, nil
	}
	if len(listed) > 32 {
		return nil, fmt.Errorf("unexpected project container count")
	}
	containers := make([]inspectedContainer, 0, len(listed))
	for _, item := range listed {
		if len(item.ID) != 12 && len(item.ID) != 64 {
			return nil, fmt.Errorf("invalid container identity")
		}
		var detail inspectedContainer
		if err := c.dockerGet(ctx, "/containers/"+url.PathEscape(item.ID)+"/json", &detail); err != nil {
			return nil, err
		}
		if err := checkLabels(detail.Config.Labels, r); err != nil {
			return nil, err
		}
		containers = append(containers, detail)
	}
	return containers, nil
}

func (c *CLI) inventory(ctx context.Context, r Record) ([]inspectedContainer, error) {
	if c.httpClient != nil && filepath.Base(c.cfg.DockerBinary) != "docker-fixture" {
		return c.inventorySocket(ctx, r)
	}
	b, err := c.run(ctx, "container", "ls", "-aq", "--filter", "label=com.docker.compose.project="+r.Project)
	if err != nil {
		return nil, err
	}
	ids := strings.Fields(string(b))
	if len(ids) == 0 {
		return nil, nil
	}
	if len(ids) > 32 {
		return nil, fmt.Errorf("unexpected project container count")
	}
	for _, id := range ids {
		if len(id) != 12 && len(id) != 64 {
			return nil, fmt.Errorf("invalid container identity")
		}
	}
	b, err = c.run(ctx, append([]string{"container", "inspect"}, ids...)...)
	if err != nil {
		return nil, err
	}
	var containers []inspectedContainer
	if err = json.Unmarshal(b, &containers); err != nil {
		return nil, fmt.Errorf("invalid container inventory")
	}
	for _, v := range containers {
		if err := checkLabels(v.Config.Labels, r); err != nil {
			return nil, err
		}
	}
	return containers, nil
}

func (c *CLI) checkVolumes(ctx context.Context, r Record) error {
	// Label-filtered inventory alone misses an unlabelled volume occupying a
	// generated name. Refuse adoption before Compose can attach it to a service.
	for _, name := range r.VolumeNames {
		listed, err := c.run(ctx, "volume", "ls", "-q", "--filter", "name=^"+regexp.QuoteMeta(name)+"$")
		if err != nil {
			return err
		}
		if strings.TrimSpace(string(listed)) == "" {
			continue
		}
		out, err := c.run(ctx, "volume", "inspect", name)
		if err != nil {
			return err
		}
		var volumes []struct{ Labels map[string]string }
		if json.Unmarshal(out, &volumes) != nil || len(volumes) != 1 {
			return fmt.Errorf("invalid volume inventory")
		}
		if err := checkLabels(volumes[0].Labels, r); err != nil {
			return err
		}
	}
	b, err := c.run(ctx, "volume", "ls", "-q", "--filter", "label=com.docker.compose.project="+r.Project)
	if err != nil {
		return err
	}
	for _, name := range strings.Fields(string(b)) {
		if !strings.HasPrefix(name, r.Project+"_") {
			return fmt.Errorf("unexpected project volume")
		}
		out, err := c.run(ctx, "volume", "inspect", name)
		if err != nil {
			return err
		}
		var volumes []struct{ Labels map[string]string }
		if json.Unmarshal(out, &volumes) != nil || len(volumes) != 1 {
			return fmt.Errorf("invalid volume inventory")
		}
		if err := checkLabels(volumes[0].Labels, r); err != nil {
			return err
		}
	}
	return nil
}
func (c *CLI) compose(ctx context.Context, r Record, file string, args ...string) error {
	_, err := c.run(ctx, append([]string{"compose", "--ansi", "never", "--env-file", "/dev/null", "--project-directory", filepath.Dir(file), "--project-name", r.Project, "--file", file}, args...)...)
	return err
}
func (c *CLI) Up(ctx context.Context, r Record, file string) error {
	if _, err := c.inventory(ctx, r); err != nil {
		return err
	}
	if err := c.checkVolumes(ctx, r); err != nil {
		return err
	}
	return c.compose(ctx, r, file, "up", "--detach", "--no-build", "--pull", "never", "--no-recreate")
}
func (c *CLI) Inspect(ctx context.Context, r Record) ([]Service, error) {
	containers, err := c.inventory(ctx, r)
	if err != nil {
		return nil, err
	}
	var services []Service
	for _, v := range containers {
		health := "unverified"
		if v.State.Health != nil {
			health = v.State.Health.Status
		}
		state := v.State.Status
		if state == "exited" && v.State.ExitCode != 0 {
			state = "failed"
		}
		ips := map[string]string{}
		for name, n := range v.NetworkSettings.Networks {
			ips[name] = n.IPAddress
		}
		services = append(services, Service{Name: v.Config.Labels["com.docker.compose.service"], State: state, Health: health, IPs: ips})
	}
	return services, nil
}
func (c *CLI) Down(ctx context.Context, r Record, file string) error {
	if _, err := c.inventory(ctx, r); err != nil {
		return err
	}
	if err := c.checkVolumes(ctx, r); err != nil {
		return err
	}
	if err := c.compose(ctx, r, file, "down", "--volumes", "--remove-orphans", "--timeout", "5"); err != nil {
		return err
	}
	containers, err := c.inventory(ctx, r)
	if err != nil {
		return err
	}
	if len(containers) != 0 {
		return fmt.Errorf("project containers remain")
	}
	b, err := c.run(ctx, "volume", "ls", "-q", "--filter", "label=com.docker.compose.project="+r.Project)
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(b)) != "" {
		return errors.New("project volumes remain")
	}
	return nil
}

// Accept stable v2 releases at or above the tested baseline. New major versions
// require explicit qualification; prereleases do not establish compatibility.
func checkComposeVersion(raw string) error {
	v := strings.TrimPrefix(strings.TrimSpace(raw), "v")
	core := strings.SplitN(v, "+", 2)[0]
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return fmt.Errorf("unrecognized Compose version; stable v2 >= %s required", ComposeVersion)
	}
	numbers := [3]int{}
	for i, p := range parts {
		if p == "" || strings.Trim(p, "0123456789") != "" {
			return fmt.Errorf("stable Compose v2 >= %s required", ComposeVersion)
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return fmt.Errorf("invalid Compose version")
		}
		numbers[i] = n
	}
	if numbers[0] != 2 || numbers[1] < 40 || numbers[1] == 40 && numbers[2] < 3 {
		return fmt.Errorf("supported Compose range: >= %s, < 3.0.0 (stable releases)", ComposeVersion)
	}
	return nil
}
