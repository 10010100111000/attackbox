// deploy-config renders single-server deployment addresses without exposing secrets.
package main

import (
	"attackbox/node"
	"bufio"
	"flag"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func webAddress(data string) (string, error) {
	values := map[string]string{"HOST": "127.0.0.1", "PORT": "3000"}
	seen := map[string]bool{}
	scanner := bufio.NewScanner(strings.NewReader(data))
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if key != "HOST" && key != "PORT" && key != "NITRO_HOST" && key != "NITRO_PORT" {
			continue
		}
		if strings.HasPrefix(key, "NITRO_") {
			return "", fmt.Errorf("use HOST and PORT instead of %s", key)
		}
		if !ok || seen[key] {
			return "", fmt.Errorf("invalid or duplicate %s", key)
		}
		seen[key] = true
		value = strings.TrimSpace(value)
		if len(value) >= 2 && (value[0] == '\'' || value[0] == '"') && value[len(value)-1] == value[0] {
			value = value[1 : len(value)-1]
		}
		values[key] = value
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("cannot read web environment")
	}
	return address(values["HOST"], values["PORT"])
}

func address(host, port string) (string, error) {
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return "", fmt.Errorf("invalid listen port")
	}
	ip := net.ParseIP(host)
	if ip == nil && host != "localhost" && host != "" {
		return "", fmt.Errorf("listen host must be an IP address or localhost")
	}
	if host == "" || host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	if host == "::" {
		host = "::1"
	}
	return net.JoinHostPort(host, strconv.Itoa(n)), nil
}

func render(env, config, template string) (string, string, string, error) {
	web, err := webAddress(env)
	if err != nil {
		return "", "", "", err
	}
	cfg, err := node.LoadNodeConfig(config)
	if err != nil {
		return "", "", "", err
	}
	host, port, err := net.SplitHostPort(cfg.Gateway.ListenAddr)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid gateway listen_addr")
	}
	worker, err := address(host, port)
	if err != nil {
		return "", "", "", err
	}
	workerURL := "http://" + worker + "/health"
	if cfg.Gateway.TLSEnabled {
		u, err := url.Parse(cfg.Gateway.PublicDataURL)
		if err != nil || u.Scheme != "wss" || u.Hostname() == "" || u.User != nil {
			return "", "", "", fmt.Errorf("TLS gateway requires a valid wss public_data_url")
		}
		u.Scheme = "https"
		u.Path = "/health"
		u.RawPath = ""
		u.RawQuery = ""
		u.Fragment = ""
		workerURL = u.String()
	}
	if strings.Count(template, "@@WEB_UPSTREAM@@") != 1 {
		return "", "", "", fmt.Errorf("nginx template must contain one WEB_UPSTREAM placeholder")
	}
	return strings.ReplaceAll(template, "@@WEB_UPSTREAM@@", web), "http://" + web + "/", workerURL, nil
}

func run() error {
	envPath := flag.String("env", "/etc/attackbox/web.env", "Web environment file")
	config := flag.String("config", "configs/node.yaml", "Worker configuration")
	template := flag.String("template", "deploy/nginx/attackbox.conf.template", "nginx template")
	out := flag.String("out", "", "Output directory")
	flag.Parse()
	if *out == "" {
		return fmt.Errorf("out is required")
	}
	env, err := os.ReadFile(*envPath)
	if err != nil {
		return fmt.Errorf("cannot read web environment file")
	}
	tmpl, err := os.ReadFile(*template)
	if err != nil {
		return err
	}
	nginx, web, worker, err := render(string(env), *config, string(tmpl))
	if err != nil {
		return err
	}
	for name, body := range map[string]string{"nginx.conf": nginx, "web-health": web + "\n", "worker-health": worker + "\n"} {
		if err := os.WriteFile(filepath.Join(*out, name), []byte(body), 0600); err != nil {
			return err
		}
	}
	return nil
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
