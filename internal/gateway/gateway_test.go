package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"attackbox/internal/config"
)

func TestLoginLimiter(t *testing.T) {
	limiter := newLoginLimiter()
	ip := "192.168.1.100"

	// 初始应允许
	if !limiter.allow(ip) {
		t.Fatal("expected allow on initial request")
	}

	// 记录 4 次失败（总计 4 次）
	for i := 0; i < 4; i++ {
		limiter.recordFailure(ip)
	}
	if !limiter.allow(ip) {
		t.Fatal("expected allow with 4 failures")
	}

	// 第 5 次失败，达到阈值
	limiter.recordFailure(ip)
	if limiter.allow(ip) {
		t.Fatal("expected block with 5 failures within 1 minute")
	}

	// 测试 reset
	limiter.reset(ip)
	if !limiter.allow(ip) {
		t.Fatal("expected allow after reset")
	}
}

func TestGetClientIP(t *testing.T) {
	gw := &Gateway{
		cfg: &config.Config{
			TrustProxy: false,
		},
	}

	req := httptest.NewRequest("GET", "/login", nil)
	req.RemoteAddr = "203.0.113.10:12345"
	req.Header.Set("CF-Connecting-IP", "198.51.100.88")
	req.Header.Set("X-Forwarded-For", "198.51.100.99, 10.0.0.1")

	// TrustProxy 为 false，必须返回直连 RemoteAddr
	ip := gw.getClientIP(req)
	if ip != "203.0.113.10" {
		t.Fatalf("expected 203.0.113.10, got %s", ip)
	}

	// TrustProxy 为 true，优先识别 CF-Connecting-IP
	gw.cfg.TrustProxy = true
	ip = gw.getClientIP(req)
	if ip != "198.51.100.88" {
		t.Fatalf("expected 198.51.100.88, got %s", ip)
	}

	// 仅有 X-Forwarded-For
	req.Header.Del("CF-Connecting-IP")
	ip = gw.getClientIP(req)
	if ip != "198.51.100.99" {
		t.Fatalf("expected 198.51.100.99, got %s", ip)
	}
}

func TestSecureHeaders(t *testing.T) {
	handler := secureHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	handler.ServeHTTP(rec, req)

	h := rec.Header()
	if h.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("missing or invalid X-Content-Type-Options: %s", h.Get("X-Content-Type-Options"))
	}
	if h.Get("X-Frame-Options") != "DENY" {
		t.Fatalf("missing or invalid X-Frame-Options: %s", h.Get("X-Frame-Options"))
	}
	if h.Get("Referrer-Policy") != "strict-origin-when-cross-origin" {
		t.Fatalf("missing or invalid Referrer-Policy: %s", h.Get("Referrer-Policy"))
	}
	if h.Get("Strict-Transport-Security") != "max-age=31536000; includeSubDomains" {
		t.Fatalf("missing or invalid HSTS: %s", h.Get("Strict-Transport-Security"))
	}
}

func TestAdminAuthMasking(t *testing.T) {
	gw := &Gateway{
		sess: map[string]*session{
			"valid-user": {
				User:    "alice",
				Role:    "user",
				Expires: time.Now().Add(time.Hour),
			},
		},
	}

	adminEndpoint := gw.adminAuth(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// 1. 匿名用户访问管理员端点，必须返回 404 NotFound 伪装不存在
	recAnon := httptest.NewRecorder()
	reqAnon := httptest.NewRequest("GET", "/api/admin/metrics", nil)
	adminEndpoint(recAnon, reqAnon)
	if recAnon.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for anonymous admin access, got %d", recAnon.Code)
	}

	// 2. 普通用户访问管理员端点，同样必须返回 404 NotFound
	recUser := httptest.NewRecorder()
	reqUser := httptest.NewRequest("GET", "/api/admin/metrics", nil)
	reqUser.AddCookie(&http.Cookie{Name: "ab", Value: "valid-user"})
	adminEndpoint(recUser, reqUser)
	if recUser.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for non-admin user access, got %d", recUser.Code)
	}
}
