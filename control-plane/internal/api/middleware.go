package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"finalproject/control-plane/internal/audit"
	"finalproject/control-plane/internal/auth"
	"finalproject/control-plane/internal/metrics"
)

func isPublic(path string) bool {
	switch path {
	case "/healthz", "/metrics":
		return true
	}
	if path == "/api/v1/auth/login" || path == "/api/v1/openapi.yaml" {
		return true
	}
	return false
}

func shouldAudit(method, path string) bool {
	if !strings.HasPrefix(path, "/api/v1/") {
		return false
	}
	if path == "/api/v1/auth/login" {
		return false
	}
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

// Wrap 注入鉴权、审计与 HTTP 指标计数。
func (s *Server) Wrap(inner http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := &audit.ResponseWriter{ResponseWriter: w}
		defer func() {
			st := rw.Status
			if st == 0 {
				st = http.StatusOK
			}
			metrics.IncHTTP(r.Method, strconv.Itoa(st))
			if shouldAudit(r.Method, r.URL.Path) && s.Audit != nil {
				actor, role := "anon", ""
				if cl, ok := auth.FromContext(r.Context()); ok {
					actor = cl.Sub
					role = cl.Role
				}
				s.Audit.Append(audit.Entry{
					Time:   time.Now().UTC(),
					Actor:  actor,
					Role:   role,
					Method: r.Method,
					Path:   r.URL.Path,
					Status: st,
				})
			}
		}()

		if isPublic(r.URL.Path) {
			inner.ServeHTTP(rw, r)
			return
		}
		if s.Auth == nil || !s.Auth.Enabled() {
			inner.ServeHTTP(rw, r.WithContext(auth.WithClaims(r.Context(), auth.AdminClaims())))
			return
		}
		cl, err := s.Auth.VerifyBearer(r.Header.Get("Authorization"))
		if err != nil {
			writeJSON(rw, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		inner.ServeHTTP(rw, r.WithContext(auth.WithClaims(r.Context(), cl)))
	})
}

func (s *Server) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	cl, ok := auth.FromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return false
	}
	if cl.Role != auth.RoleAdmin {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "admin only"})
		return false
	}
	return true
}

func metricsSnapshot(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	c2, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if fn := metricsSnapshotHook; fn != nil {
		fn(c2)
	}
}

// metricsSnapshotHook 由 main 注入 ledger / store / cluster。
var metricsSnapshotHook func(context.Context)

func SetMetricsSnapshotHook(fn func(context.Context)) {
	metricsSnapshotHook = fn
}
