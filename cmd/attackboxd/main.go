// Command attackboxd is the single-port gateway: TLS on 443, token auth,
// routing, session expiry, and the two bridges (RFB unix socket and pty).
package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"attackbox/internal/box"
	"attackbox/internal/config"
	"attackbox/internal/gateway"
)

func main() {
	cfgPath := flag.String("config", "/etc/attackbox/config.json", "config file")
	listen := flag.String("listen", "", "override listen address, e.g. :8443")
	flag.Parse()

	c, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	if *listen != "" {
		c.Listen = *listen
	}

	if err := box.EnsureHostNetwork(); err != nil {
		log.Printf("warning: init host network and firewall: %v", err)
	}

	if err := box.EnsureBaseRootfs(c.BaseRootfs); err != nil {
		log.Printf("warning: init base rootfs: %v", err)
	}

	mgr := box.NewManagerWithConfig(c.BoxesDir, c.BaseRootfs)

	gw, err := gateway.New(c, mgr)
	if err != nil {
		log.Fatalf("gateway: %v", err)
	}

	// 启动对账：从 SQLite 恢复活跃沙箱租约，仅清理死掉的沙箱或无租约残留，重启绝不踢下线
	gw.Reconcile(context.Background())

	go gw.Run(context.Background())

	srv := &http.Server{
		Addr:              c.Listen,
		Handler:           withLogging(gw.Handler(), c.TrustProxy),
		ReadHeaderTimeout: 10 * time.Second,
		// A WebSocket upgrade only exists on HTTP/1.1. Go enables HTTP/2
		// automatically for TLS servers, and Cloudflare then negotiates h2
		// over ALPN and the handshake dies with HTTP/2 400.
		// An empty non-nil TLSNextProto switches h2 off.
		TLSNextProto: map[string]func(*http.Server, *tls.Conn, http.Handler){},
	}

	log.Printf("attackboxd listening on %s (cert %s, %d boxes, web %s)",
		c.Listen, c.Cert, len(c.Boxes), c.WebDir)
	log.Fatal(srv.ListenAndServeTLS(c.Cert, c.Key))
}

type statusWriter struct {
	http.ResponseWriter
	code int
}

func (s *statusWriter) WriteHeader(c int) {
	s.code = c
	s.ResponseWriter.WriteHeader(c)
}

// Hijack keeps gorilla/websocket able to take over the connection.
func (s *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := s.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("response writer does not support hijacking")
	}
	return h.Hijack()
}

func withLogging(h http.Handler, trustProxy bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sw := &statusWriter{ResponseWriter: w, code: 200}
		start := time.Now()
		h.ServeHTTP(sw, r)
		log.Printf("%s %s host=%s status=%d dur=%s ip=%s ua=%q",
			sanitizeLog(r.Method), sanitizeLog(r.URL.RequestURI()), sanitizeLog(r.Host), sw.code,
			time.Since(start).Truncate(time.Millisecond),
			sanitizeLog(clientIP(r, trustProxy)), trim(sanitizeLog(r.UserAgent()), 70))
	})
}

// clientIP checks reverse proxy headers only if trustProxy is enabled in config.
func clientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if v := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); v != "" {
			return v
		}
		if v := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); v != "" {
			parts := strings.Split(v, ",")
			return strings.TrimSpace(parts[0])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func sanitizeLog(s string) string {
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}

func trim(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
