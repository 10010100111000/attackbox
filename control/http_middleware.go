package control

import (
	"attackbox/protocol"
	"bufio"
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"time"
)

type contextKey string

const (
	contextKeyUserID    contextKey = "user_id"
	contextKeyUsername  contextKey = "username"
	contextKeyRole      contextKey = "role"
	contextKeyRequestID contextKey = "request_id"
)

func UserFromContext(ctx context.Context) (int64, string, string, bool) {
	uid, ok1 := ctx.Value(contextKeyUserID).(int64)
	user, ok2 := ctx.Value(contextKeyUsername).(string)
	role, ok3 := ctx.Value(contextKeyRole).(string)
	return uid, user, role, ok1 && ok2 && ok3
}

func (s *ControlHTTPServer) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			writeError(w, http.StatusUnauthorized, "missing or malformed Authorization header")
			return
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := VerifyJWT(s.pubKey, tokenStr)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}

		if s.db != nil {
			u, err := s.db.GetUserByID(claims.Sub)
			if err != nil || u == nil || u.Status != 1 || u.TokenVersion != claims.TokenVersion {
				writeError(w, http.StatusUnauthorized, "session revoked or account inactive")
				return
			}
		}

		ctx := context.WithValue(r.Context(), contextKeyUserID, claims.Sub)
		ctx = context.WithValue(ctx, contextKeyUsername, claims.Username)
		ctx = context.WithValue(ctx, contextKeyRole, claims.Role)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *ControlHTTPServer) corsMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	originsMap := make(map[string]bool)
	for _, o := range allowedOrigins {
		originsMap[strings.TrimSpace(o)] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if originsMap[origin] || len(allowedOrigins) == 0 || allowedOrigins[0] == "*" {
				if origin != "" {
					w.Header().Set("Access-Control-Allow-Origin", origin)
				} else {
					w.Header().Set("Access-Control-Allow-Origin", "*")
				}
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With, X-Request-ID")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

type statusResponseWriter struct {
	http.ResponseWriter
	statusCode int
	written    int64
}

func newStatusResponseWriter(w http.ResponseWriter) *statusResponseWriter {
	return &statusResponseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}
}

func (rw *statusResponseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *statusResponseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.written += int64(n)
	return n, err
}

func (rw *statusResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hj, ok := rw.ResponseWriter.(http.Hijacker); ok {
		return hj.Hijack()
	}
	return nil, nil, fmt.Errorf("underlying ResponseWriter does not implement http.Hijacker")
}

func (rw *statusResponseWriter) Flush() {
	if fl, ok := rw.ResponseWriter.(http.Flusher); ok {
		fl.Flush()
	}
}

var reqIDCounter uint64

func generateRequestID() string {
	id := atomic.AddUint64(&reqIDCounter, 1)
	return fmt.Sprintf("req-%08x-%04x", time.Now().Unix(), id&0xffff)
}

func (s *ControlHTTPServer) requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get("X-Request-ID")
		if reqID == "" {
			reqID = generateRequestID()
		}
		w.Header().Set("X-Request-ID", reqID)
		ctx := context.WithValue(r.Context(), contextKeyRequestID, reqID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *ControlHTTPServer) recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				reqID, _ := r.Context().Value(contextKeyRequestID).(string)
				if reqID == "" {
					reqID = "unknown"
				}
				stack := debug.Stack()
				log.Printf("[PANIC] [%s] %s %s: %v\n%s", reqID, r.Method, r.URL.Path, rec, string(stack))
				writeError(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *ControlHTTPServer) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/agent/connect") {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		srw := newStatusResponseWriter(w)
		next.ServeHTTP(srw, r)
		reqID, _ := r.Context().Value(contextKeyRequestID).(string)
		if reqID == "" {
			reqID = "none"
		}
		log.Printf("[HTTP] [%s] %d %s %s (%v)", reqID, srw.statusCode, r.Method, r.URL.Path, time.Since(start))
	})
}

// bodyLimitMiddleware enforces protocol.MaxHTTPPayloadSize (1MB) on incoming HTTP request bodies.
func (s *ControlHTTPServer) bodyLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil && !strings.HasPrefix(r.URL.Path, "/api/agent/connect") {
			r.Body = http.MaxBytesReader(w, r.Body, protocol.MaxHTTPPayloadSize)
		}
		next.ServeHTTP(w, r)
	})
}
