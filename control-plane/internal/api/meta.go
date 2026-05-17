package api

import (
	_ "embed"
	"encoding/json"
	"net/http"
	"strconv"

	"finalproject/control-plane/internal/auth"
	"finalproject/control-plane/internal/metrics"
)

//go:embed spec/openapi.yaml
var openAPISpec []byte

type loginBody struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body loginBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	cl, ok := auth.TryLogin(body.Username, body.Password)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}
	if s.Auth == nil || !s.Auth.Enabled() {
		writeJSON(w, http.StatusOK, map[string]any{
			"access_token": "",
			"token_type":   "Bearer",
			"expires_in":   0,
			"note":         "CP_AUTH_DISABLED=1: bearer 未签发，请求将以内置 admin 身份处理",
		})
		return
	}
	tok, err := s.Auth.Sign(cl)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": tok,
		"token_type":   "Bearer",
		"expires_in":   86400,
	})
}

func (s *Server) handleAuthMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	cl, ok := auth.FromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"sub":    cl.Sub,
		"role":   cl.Role,
		"tenant": cl.Tenant,
	})
}

func (s *Server) handleOpenAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	_, _ = w.Write(openAPISpec)
}

func (s *Server) handleAuditLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.requireAdmin(w, r) {
		return
	}
	if s.Audit == nil {
		writeJSON(w, http.StatusOK, map[string]any{"entries": []any{}})
		return
	}
	n := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if x, err := strconv.Atoi(v); err == nil && x > 0 && x <= 500 {
			n = x
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": s.Audit.Recent(n)})
}

func (s *Server) handleObservabilitySummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	metricsSnapshot(r.Context())
	out := map[string]any{
		"prometheus": map[string]string{
			"control_plane_metrics": "/metrics",
			"node_kernel_exporter": "见 cmd/node-kernel-exporter（节点级补充）",
		},
		"gauges_hint": "与 /metrics 中 cp_* 系列一致，便于前端无 Prom 时的降级展示",
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	metricsSnapshot(r.Context())
	metrics.Handler(w, r)
}

func (s *Server) registerMetaRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/auth/login", s.method("POST", s.handleAuthLogin))
	mux.HandleFunc("/api/v1/auth/me", s.method("GET", s.handleAuthMe))
	mux.HandleFunc("/api/v1/openapi.yaml", s.method("GET", s.handleOpenAPI))
	mux.HandleFunc("/api/v1/audit/logs", s.method("GET", s.handleAuditLogs))
	mux.HandleFunc("/api/v1/observability/summary", s.method("GET", s.handleObservabilitySummary))
	mux.HandleFunc("/metrics", s.method("GET", s.handleMetrics))
}
