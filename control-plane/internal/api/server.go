package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"finalproject/control-plane/internal/audit"
	"finalproject/control-plane/internal/auth"
	"finalproject/control-plane/internal/ledger"
	"finalproject/control-plane/internal/model"
	"finalproject/control-plane/internal/orchestration"
)

type Server struct {
	Ledger       *ledger.Ledger
	Cluster      *orchestration.Cluster
	Workloads    *orchestration.WorkloadStore
	Orchestrator *orchestration.OrchestratorService
	Auth         *auth.Config
	Audit        *audit.Ring
	Idempo       *IdempotencyStore
}

func (s *Server) Register(mux *http.ServeMux) {
	s.registerMetaRoutes(mux)
	mux.HandleFunc("/healthz", s.method("GET", s.handleHealth))
	mux.HandleFunc("/api/v1/catalog/flavors", s.method("GET", s.handleListFlavors))
	mux.HandleFunc("/api/v1/catalog/flavors/", s.method("GET", s.handleFlavorByID))
	mux.HandleFunc("/api/v1/pools", s.method("GET", s.handleListPools))
	mux.HandleFunc("/api/v1/pools/", s.method("GET", s.handlePoolByID))
	mux.HandleFunc("/api/v1/inventory/nodes", s.method("GET", s.handleListNodes))
	mux.HandleFunc("/api/v1/inventory/nodes/", s.method("GET", s.handleNodeByName))
	mux.HandleFunc("/api/v1/ledger/summary", s.method("GET", s.handleLedgerSummary))
	mux.HandleFunc("/api/v1/k8s/bindings/", s.method("GET", s.handleK8sBinding))
	mux.HandleFunc("/api/v1/allocations", s.routeAllocationsRoot)
	mux.HandleFunc("/api/v1/allocations/", s.routeAllocationsPrefix)
	s.registerWorkloads(mux)
}

func (s *Server) method(allowed string, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != allowed {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		h(w, r)
	}
}

func (s *Server) routeAllocationsRoot(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleListAllocations(w, r)
	case http.MethodPost:
		s.handleCreateAllocation(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) routeAllocationsPrefix(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/allocations/")
	if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/release") {
		s.handleAllocationSubpath(w, r)
		return
	}
	if r.Method == http.MethodGet && path != "" && !strings.Contains(path, "/") {
		s.handleGetAllocation(w, r)
		return
	}
	http.NotFound(w, r)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	writeJSONStatus(w, status, v)
}

func writeJSONStatus(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) handleListFlavors(w http.ResponseWriter, r *http.Request) {
	c := s.Ledger.Catalog()
	writeJSON(w, http.StatusOK, c.Flavors)
}

func (s *Server) handleFlavorByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/catalog/flavors/")
	if id == "" {
		http.NotFound(w, r)
		return
	}
	f, ok := s.Ledger.Catalog().FlavorByID(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "flavor not found"})
		return
	}
	writeJSON(w, http.StatusOK, f)
}

func (s *Server) handleListPools(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.Ledger.Catalog().Pools)
}

func (s *Server) handlePoolByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/pools/")
	if id == "" {
		http.NotFound(w, r)
		return
	}
	p, ok := s.Ledger.Catalog().PoolByID(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "pool not found"})
		return
	}
	if r.URL.Query().Get("expand") == "flavors" {
		flavors := make([]any, 0, len(p.SupportedFlavorIDs))
		for _, fid := range p.SupportedFlavorIDs {
			if f, ok := s.Ledger.Catalog().FlavorByID(fid); ok {
				flavors = append(flavors, f)
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"pool": p, "flavors": flavors})
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) handleListNodes(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.Ledger.ListNodes())
}

func (s *Server) handleNodeByName(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/api/v1/inventory/nodes/")
	if name == "" {
		http.NotFound(w, r)
		return
	}
	n, ok := s.Ledger.GetNode(name)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "node not found"})
		return
	}
	writeJSON(w, http.StatusOK, n)
}

func (s *Server) handleLedgerSummary(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.Ledger.Summary())
}

func (s *Server) handleK8sBinding(w http.ResponseWriter, r *http.Request) {
	flavorID := strings.TrimPrefix(r.URL.Path, "/api/v1/k8s/bindings/")
	if flavorID == "" {
		http.NotFound(w, r)
		return
	}
	b, err := s.Ledger.K8sBindingForFlavor(flavorID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, b)
}

type createAllocBody struct {
	Tenant    string `json:"tenant"`
	PoolID    string `json:"pool_id"`
	FlavorID  string `json:"flavor_id"`
	Namespace string `json:"namespace"`
	PodRef    string `json:"pod_ref"`
}

func (s *Server) handleCreateAllocation(w http.ResponseWriter, r *http.Request) {
	cl, ok := auth.FromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	var body createAllocBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if !cl.CanMutateTenant(body.Tenant) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "tenant mismatch"})
		return
	}
	a, err := s.Ledger.CreateAllocation(ledger.CreateAllocationInput{
		Tenant:    body.Tenant,
		PoolID:    body.PoolID,
		FlavorID:  body.FlavorID,
		Namespace: body.Namespace,
		PodRef:    body.PodRef,
	})
	if err != nil {
		status := http.StatusBadRequest
		if err == ledger.ErrNoCapacity {
			status = http.StatusConflict
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	flavor, _ := s.Ledger.Catalog().FlavorByID(a.FlavorID)
	resp := map[string]any{
		"allocation": a,
		"k8s": map[string]any{
			"extended_resource": flavor.K8sExtendedResource,
			"limits":            flavor.PodResourceLimits(),
			"hami_template":     flavor.AVITemplateID,
			"provision_mode":    flavor.ProvisionMode,
		},
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (s *Server) handleListAllocations(w http.ResponseWriter, r *http.Request) {
	cl, ok := auth.FromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	all := s.Ledger.ListAllocations()
	if cl.Role == auth.RoleTenant {
		filtered := make([]*model.Allocation, 0, len(all))
		for _, a := range all {
			if a != nil && cl.TenantMatches(a.Tenant) {
				filtered = append(filtered, a)
			}
		}
		all = filtered
	}
	writeJSON(w, http.StatusOK, all)
}

func (s *Server) handleGetAllocation(w http.ResponseWriter, r *http.Request) {
	cl, ok := auth.FromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/allocations/")
	if id == "" || strings.Contains(id, "/") {
		http.NotFound(w, r)
		return
	}
	a, has := s.Ledger.GetAllocation(id)
	if !has {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "allocation not found"})
		return
	}
	if !cl.TenantMatches(a.Tenant) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	writeJSON(w, http.StatusOK, a)
}

func (s *Server) handleAllocationSubpath(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/allocations/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		http.NotFound(w, r)
		return
	}
	id, action := parts[0], parts[1]
	if r.Method == http.MethodPost && action == "release" {
		cl, ok := auth.FromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		if a0, ok0 := s.Ledger.GetAllocation(id); ok0 && !cl.TenantMatches(a0.Tenant) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
			return
		}
		if err := s.Ledger.ReleaseAllocation(id); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		a, _ := s.Ledger.GetAllocation(id)
		writeJSON(w, http.StatusOK, a)
		return
	}
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}
