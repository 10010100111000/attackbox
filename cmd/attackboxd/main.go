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

	gw, err := gateway.New(c, box.NewManagerWithConfig(c.BoxesDir, c.BaseRootfs))
	if err != nil {
		log.Fatalf("gateway: %v", err)
	}

	go gw.Run(context.Background())

	srv := &http.Server{
		Addr:              c.Listen,
		Handler:           withLogging(gw.Handler()),
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

func withLogging(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sw := &statusWriter{ResponseWriter: w, code: 200}
		start := time.Now()
		h.ServeHTTP(sw, r)
		log.Printf("%s %s host=%s status=%d dur=%s ip=%s ua=%q",
			r.Method, r.URL.RequestURI(), r.Host, sw.code,
			time.Since(start).Truncate(time.Millisecond),
			clientIP(r), trim(r.UserAgent(), 70))
	})
}

// clientIP prefers the header Cloudflare sets, so the log shows the real user.
func clientIP(r *http.Request) string {
	if v := r.Header.Get("CF-Connecting-IP"); v != "" {
		return v
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func trim(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
