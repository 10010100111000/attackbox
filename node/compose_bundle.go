package node

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var bundleIDPattern = regexp.MustCompile(`^[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}$`)
var bundleFilePattern = regexp.MustCompile(`^[A-Za-z0-9_.\-/]+$`)

func bundleFilename(name string) bool {
	if !bundleFilePattern.MatchString(name) || strings.HasPrefix(name, "/") || len(name) > 240 {
		return false
	}
	for _, p := range strings.Split(name, "/") {
		if p == "" || p == "." || p == ".." {
			return false
		}
	}
	return true
}
func bundleDigest(data []byte) string { h := sha256.Sum256(data); return hex.EncodeToString(h[:]) }
func downloadComposeBundle(ctx context.Context, cfg *NodeConfig, id, hash string) (string, error) {
	if !bundleIDPattern.MatchString(id) || !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(hash) {
		return "", fmt.Errorf("invalid bundle identity")
	}
	u, err := url.Parse(cfg.Node.ControlURL)
	if err != nil {
		return "", fmt.Errorf("invalid controller URL")
	}
	switch u.Scheme {
	case "wss":
		u.Scheme = "https"
	case "ws":
		u.Scheme = "http"
	default:
		return "", fmt.Errorf("invalid controller transport")
	}
	if u.User != nil || u.Scheme != "https" && u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost" && u.Hostname() != "::1" {
		return "", fmt.Errorf("bundle download requires TLS")
	}
	u.Path = "/api/worker/lab-bundles/" + id
	u.RawQuery = ""
	u.Fragment = ""
	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.Node.ClusterSecret)
	req.Header.Set("X-Node-ID", cfg.Node.NodeID)
	client := &http.Client{Timeout: 60 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("bundle download failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("bundle download rejected")
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, (48<<20)+1))
	if err != nil || len(data) > 48<<20 {
		return "", fmt.Errorf("bundle download exceeds limit")
	}
	var pack struct {
		ID    string            `json:"id"`
		Hash  string            `json:"hash"`
		Files map[string]string `json:"files"`
	}
	if json.Unmarshal(data, &pack) != nil || pack.ID != id || pack.Hash != hash || len(pack.Files) == 0 || len(pack.Files) > 512 {
		return "", fmt.Errorf("bundle response invalid")
	}
	files := map[string][]byte{}
	var keys []string
	total := 0
	for name, encoded := range pack.Files {
		if !bundleFilename(name) || len(encoded) > ((4<<20)+2)/3*4 {
			return "", fmt.Errorf("bundle entry invalid")
		}
		b, err := base64.StdEncoding.Strict().DecodeString(encoded)
		total += len(b)
		if err != nil || len(b) > 4<<20 || total > 32<<20 {
			return "", fmt.Errorf("bundle entry exceeds limit")
		}
		files[name] = b
		keys = append(keys, name)
	}
	sort.Strings(keys)
	var manifest strings.Builder
	for _, name := range keys {
		manifest.WriteString(name + "\x00" + bundleDigest(files[name]) + "\n")
	}
	if bundleDigest([]byte(manifest.String())) != hash {
		return "", fmt.Errorf("bundle digest mismatch")
	}
	root := filepath.Join(cfg.ComposeLabs.StateDir, "incoming")
	if err := os.MkdirAll(root, 0700); err != nil {
		return "", err
	}
	tmp, err := os.MkdirTemp(root, "bundle-")
	if err != nil {
		return "", err
	}
	ok := false
	defer func() {
		if !ok {
			os.RemoveAll(tmp)
		}
	}()
	for _, name := range keys {
		path := filepath.Join(tmp, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return "", err
		}
		if err := os.WriteFile(path, files[name], 0600); err != nil {
			return "", err
		}
	}
	ok = true
	return tmp, nil
}
