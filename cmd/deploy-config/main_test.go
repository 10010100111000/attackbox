package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWebAddress(t *testing.T) {
	for _, tc := range []struct{ env, want string }{
		{"HOST=127.0.0.1\nPORT=3107\nRESEND_API_KEY=private", "127.0.0.1:3107"},
		{"HOST=0.0.0.0\nPORT='4100'", "127.0.0.1:4100"},
		{"HOST=\"::\"\nPORT=3200", "[::1]:3200"},
		{"", "127.0.0.1:3000"},
	} {
		got, err := webAddress(tc.env)
		if err != nil || got != tc.want {
			t.Fatalf("got %q, %v; want %q", got, err, tc.want)
		}
	}
	for _, env := range []string{"PORT=0", "PORT=65536", "PORT=3000;echo x", "HOST=bad/path", "NITRO_PORT=4000", "PORT=3001\nPORT=3002"} {
		if _, err := webAddress(env); err == nil {
			t.Fatalf("accepted invalid config %q", env)
		}
	}
}

func TestRenderUsesActualPorts(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "node.yaml")
	for _, tc := range []struct{ yaml, want string }{
		{"gateway:\n  listen_addr: '[::]:9543'\n", "http://[::1]:9543/health"},
		{"gateway:\n  listen_addr: '0.0.0.0:9543'\n  tls_enabled: true\n  public_data_url: 'wss://worker.example.org:9543'\n", "https://worker.example.org:9543/health"},
	} {
		if err := os.WriteFile(cfg, []byte(tc.yaml), 0600); err != nil {
			t.Fatal(err)
		}
		nginx, web, worker, err := render("PORT=3101\nRESEND_API_KEY=secret", cfg, "proxy_pass http://@@WEB_UPSTREAM@@;")
		if err != nil {
			t.Fatal(err)
		}
		if nginx != "proxy_pass http://127.0.0.1:3101;" || web != "http://127.0.0.1:3101/" || worker != tc.want || strings.Contains(nginx, "secret") {
			t.Fatal(nginx, web, worker)
		}
	}
}
