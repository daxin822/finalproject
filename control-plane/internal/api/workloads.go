package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"

	"finalproject/control-plane/internal/auth"
	"finalproject/control-plane/internal/catalog"
	"finalproject/control-plane/internal/model"
	"finalproject/control-plane/internal/orchestration"
)

func (s *Server) registerWorkloads(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/workloads", s.handleWorkloadsCollection)
	mux.HandleFunc("/api/v1/workloads/", s.handleWorkloadsSubroutes)
}

func (s *Server) handleWorkloadsCollection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.Workloads == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "workload store not configured"})
		return
	}
	tenant := r.URL.Query().Get("tenant")
	phaseStr := r.URL.Query().Get("phase")
	if cl, ok := auth.FromContext(r.Context()); ok && cl.Role == auth.RoleTenant {
		if tenant != "" && tenant != cl.Tenant {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "cannot query other tenant"})
			return
		}
		tenant = cl.Tenant
	}
	var phase model.WorkloadPhase
	if phaseStr != "" {
		phase = model.WorkloadPhase(phaseStr)
	}
	list := s.Workloads.List(tenant, phase)
	writeJSON(w, http.StatusOK, map[string]any{"workloads": list})
}

func (s *Server) handleWorkloadsSubroutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/workloads/")
	switch {
	case path == "training/render" && r.Method == http.MethodPost:
		s.handleRenderTraining(w, r)
		return
	case path == "training" && r.Method == http.MethodPost:
		s.handleSubmitTraining(w, r)
		return
	case path == "inference/render" && r.Method == http.MethodPost:
		s.handleRenderInference(w, r)
		return
	case path == "inference" && r.Method == http.MethodPost:
		s.handleSubmitInference(w, r)
		return
	case path == "apply" && r.Method == http.MethodPost:
		s.handleApplyManifest(w, r)
		return
	case path == "pods/watch" && r.Method == http.MethodGet:
		s.handleWatchWorkloadPods(w, r)
		return
	case path == "pods" && r.Method == http.MethodGet:
		s.handleListWorkloadPods(w, r)
		return
	case path == "cluster" && r.Method == http.MethodGet:
		s.handleWorkloadCluster(w, r)
		return
	case strings.HasSuffix(path, "/watch") && r.Method == http.MethodGet:
		id := strings.TrimSuffix(path, "/watch")
		if id == "" || strings.Contains(id, "/") {
			http.NotFound(w, r)
			return
		}
		s.handleWatchWorkloadRecord(w, r, id)
		return
	default:
		if strings.Contains(path, "/") {
			http.NotFound(w, r)
			return
		}
		id := path
		switch r.Method {
		case http.MethodGet:
			s.handleGetWorkload(w, r, id)
		case http.MethodDelete:
			s.handleDeleteWorkload(w, r, id)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func applyFlavorToNPUResources(f catalog.SliceFlavor, extendedRes, memoryRes *string, memoryQty *string, req map[string]string) {
	if *extendedRes == "" {
		*extendedRes = f.K8sExtendedResource
	}
	if f.K8sMemoryExtendedResource != "" && f.MemoryMiBRequest > 0 {
		if *memoryRes == "" {
			*memoryRes = f.K8sMemoryExtendedResource
		}
		if *memoryQty == "" {
			*memoryQty = fmt.Sprintf("%d", f.MemoryMiBRequest)
		}
		if req != nil {
			req[f.K8sExtendedResource] = "1"
			req[f.K8sMemoryExtendedResource] = *memoryQty
		}
	}
}

func (s *Server) enrichTrainingFromCatalog(in *orchestration.TrainingWorkloadSpec) {
	if in == nil || in.FlavorID == "" {
		return
	}
	f, ok := s.Ledger.Catalog().FlavorByID(in.FlavorID)
	if !ok {
		return
	}
	applyFlavorToNPUResources(f, &in.ExtendedRes, &in.ExtendedResMemory, &in.MemoryQuantity, nil)
}

func (s *Server) enrichInferenceFromCatalog(in *orchestration.InferenceWorkloadSpec) {
	if in == nil || in.FlavorID == "" {
		return
	}
	f, ok := s.Ledger.Catalog().FlavorByID(in.FlavorID)
	if !ok {
		return
	}
	if in.ResourceRequests == nil {
		in.ResourceRequests = map[string]string{}
	}
	applyFlavorToNPUResources(f, &in.ExtendedRes, &in.ExtendedResMemory, &in.MemoryQuantity, in.ResourceRequests)
}

func (s *Server) handleRenderTraining(w http.ResponseWriter, r *http.Request) {
	var in orchestration.TrainingWorkloadSpec
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	s.enrichTrainingFromCatalog(&in)
	if err := validateTrainingSpec(in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	yml, err := orchestration.RenderTrainingYAML(in)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"yaml": yml, "spec": in})
}

func (s *Server) handleRenderInference(w http.ResponseWriter, r *http.Request) {
	var in orchestration.InferenceWorkloadSpec
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	s.enrichInferenceFromCatalog(&in)
	if err := validateInferenceSpec(in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	yml, err := orchestration.RenderInferenceYAML(in)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"yaml": yml, "spec": in})
}

type applyManifestBody struct {
	Manifest string `json:"manifest"`
}

func (s *Server) handleApplyManifest(w http.ResponseWriter, r *http.Request) {
	if s.Cluster == nil || !s.Cluster.Enabled() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "kubernetes client not configured"})
		return
	}
	var body applyManifestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if strings.TrimSpace(body.Manifest) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "manifest required"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	applied, err := s.Cluster.ApplyYAML(ctx, body.Manifest)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error(), "applied": applied})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"applied": applied})
}

func (s *Server) handleListWorkloadPods(w http.ResponseWriter, r *http.Request) {
	if s.Cluster == nil || !s.Cluster.Enabled() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "kubernetes client not configured"})
		return
	}
	ns := r.URL.Query().Get("namespace")
	sel := r.URL.Query().Get("labelSelector")
	if ns == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "namespace query required"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	pods, err := s.Cluster.ListPods(ctx, ns, sel)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"pods": pods})
}

func (s *Server) handleWatchWorkloadPods(w http.ResponseWriter, r *http.Request) {
	if s.Cluster == nil || !s.Cluster.Enabled() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "kubernetes client not configured"})
		return
	}
	ns := r.URL.Query().Get("namespace")
	sel := r.URL.Query().Get("labelSelector")
	if ns == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "namespace query required"})
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "streaming unsupported"})
		return
	}
	wi, err := s.Cluster.WatchPods(r.Context(), ns, metav1.ListOptions{LabelSelector: sel})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	defer wi.Stop()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	for {
		select {
		case <-r.Context().Done():
			return
		case ev, ok := <-wi.ResultChan():
			if !ok {
				return
			}
			if ev.Type == watch.Error {
				b, _ := json.Marshal(map[string]string{"error": "watch stream error"})
				_, _ = fmt.Fprintf(w, "data: %s\n\n", string(b))
				flusher.Flush()
				return
			}
			pod, ok := ev.Object.(*corev1.Pod)
			if !ok {
				continue
			}
			payload := map[string]any{
				"type":   string(ev.Type),
				"name":   pod.Name,
				"phase":  string(pod.Status.Phase),
				"node":   pod.Spec.NodeName,
				"reason": pod.Status.Reason,
			}
			b, _ := json.Marshal(payload)
			_, _ = fmt.Fprintf(w, "data: %s\n\n", string(b))
			flusher.Flush()
		}
	}
}

func (s *Server) handleWorkloadCluster(w http.ResponseWriter, r *http.Request) {
	if s.Cluster == nil || !s.Cluster.Enabled() {
		writeJSON(w, http.StatusOK, map[string]any{"kubernetes": false})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	groups, err := s.Cluster.DiscoveryGroups(ctx)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"kubernetes": true, "discovery_error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"kubernetes": true, "api_groups": groups})
}

func (s *Server) handleSubmitTraining(w http.ResponseWriter, r *http.Request) {
	if s.Orchestrator == nil || s.Cluster == nil || !s.Cluster.Enabled() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "kubernetes client not configured"})
		return
	}
	cl, ok := auth.FromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "read body"})
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	if key := idempotencyKey(r); key != "" && s.Idempo != nil {
		if st, out, hit := s.Idempo.Lookup(key, bodyBytes); hit {
			replayJSON(w, st, out)
			return
		}
	}
	var in orchestration.TrainingWorkloadSpec
	if err := json.Unmarshal(bodyBytes, &in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if strings.TrimSpace(in.Tenant) == "" && cl.Role == auth.RoleTenant {
		in.Tenant = cl.Tenant
	}
	if !cl.CanMutateTenant(in.Tenant) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "tenant mismatch"})
		return
	}
	s.enrichTrainingFromCatalog(&in)
	if err := validateTrainingSpec(in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	rec, err := s.Orchestrator.SubmitTraining(ctx, in)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	resp := map[string]any{
		"job_id":   rec.ID,
		"phase":    rec.Phase,
		"workload": rec,
	}
	raw, _ := json.Marshal(resp)
	if key := idempotencyKey(r); key != "" && s.Idempo != nil {
		s.Idempo.Store(key, bodyBytes, http.StatusCreated, raw)
	}
	writeJSONStatus(w, http.StatusCreated, resp)
}

func (s *Server) handleSubmitInference(w http.ResponseWriter, r *http.Request) {
	if s.Orchestrator == nil || s.Cluster == nil || !s.Cluster.Enabled() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "kubernetes client not configured"})
		return
	}
	cl, ok := auth.FromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "read body"})
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	if key := idempotencyKey(r); key != "" && s.Idempo != nil {
		if st, out, hit := s.Idempo.Lookup(key, bodyBytes); hit {
			replayJSON(w, st, out)
			return
		}
	}
	var in orchestration.InferenceWorkloadSpec
	if err := json.Unmarshal(bodyBytes, &in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if strings.TrimSpace(in.Tenant) == "" && cl.Role == auth.RoleTenant {
		in.Tenant = cl.Tenant
	}
	if !cl.CanMutateTenant(in.Tenant) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "tenant mismatch"})
		return
	}
	s.enrichInferenceFromCatalog(&in)
	if err := validateInferenceSpec(in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	rec, err := s.Orchestrator.SubmitInference(ctx, in)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	serviceID := ""
	if in.CreateService {
		serviceID = rec.Name
	}
	resp := map[string]any{
		"service_id": serviceID,
		"workload":   rec,
	}
	raw, _ := json.Marshal(resp)
	if key := idempotencyKey(r); key != "" && s.Idempo != nil {
		s.Idempo.Store(key, bodyBytes, http.StatusCreated, raw)
	}
	writeJSONStatus(w, http.StatusCreated, resp)
}

func (s *Server) handleGetWorkload(w http.ResponseWriter, r *http.Request, id string) {
	if s.Workloads == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "workload store not configured"})
		return
	}
	cl, okClaims := auth.FromContext(r.Context())
	if !okClaims {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	rec, has := s.Workloads.Get(id)
	if !has {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "workload not found"})
		return
	}
	if cl.Role == auth.RoleTenant && rec.Tenant != "" && !cl.TenantMatches(rec.Tenant) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	if s.Cluster != nil && s.Cluster.Enabled() && rec.Namespace != "" && rec.Name != "" {
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		if ev, err := s.Cluster.FetchEventsSummary(ctx, rec.Namespace, rec.Name, 5); err == nil && len(ev) > 0 {
			rec.EventsSummary = ev
		}
	}
	writeJSON(w, http.StatusOK, rec)
}

func (s *Server) handleDeleteWorkload(w http.ResponseWriter, r *http.Request, id string) {
	if s.Orchestrator == nil || s.Workloads == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "orchestrator not configured"})
		return
	}
	cl, ok := auth.FromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	if rec, ok := s.Workloads.Get(id); ok && cl.Role == auth.RoleTenant && rec.Tenant != "" && !cl.TenantMatches(rec.Tenant) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()
	if err := s.Orchestrator.DeleteWorkload(ctx, id); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "id": id})
}

func (s *Server) handleWatchWorkloadRecord(w http.ResponseWriter, r *http.Request, id string) {
	if s.Workloads == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "workload store not configured"})
		return
	}
	cl, okClaims := auth.FromContext(r.Context())
	if !okClaims {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "streaming unsupported"})
		return
	}
	rec, has := s.Workloads.Get(id)
	if !has {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "workload not found"})
		return
	}
	if cl.Role == auth.RoleTenant && rec.Tenant != "" && !cl.TenantMatches(rec.Tenant) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}
	ch := s.Workloads.Subscribe(id)
	defer s.Workloads.Unsubscribe(id, ch)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	send := func(wr *model.WorkloadRecord) {
		b, _ := json.Marshal(wr)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", string(b))
		flusher.Flush()
	}
	send(rec)

	for {
		select {
		case <-r.Context().Done():
			return
		case wr, ok := <-ch:
			if !ok {
				return
			}
			if wr != nil {
				send(wr)
			}
		}
	}
}

func validateTrainingSpec(in orchestration.TrainingWorkloadSpec) error {
	if strings.TrimSpace(in.Name) == "" {
		return fmt.Errorf("name required")
	}
	if strings.TrimSpace(in.Namespace) == "" {
		return fmt.Errorf("namespace required")
	}
	if strings.TrimSpace(in.Image) == "" {
		return fmt.Errorf("image required")
	}
	if strings.TrimSpace(in.ExtendedRes) == "" {
		return fmt.Errorf("extended_resource or flavor_id required")
	}
	return nil
}

func validateInferenceSpec(in orchestration.InferenceWorkloadSpec) error {
	if strings.TrimSpace(in.Name) == "" {
		return fmt.Errorf("name required")
	}
	if strings.TrimSpace(in.Namespace) == "" {
		return fmt.Errorf("namespace required")
	}
	if strings.TrimSpace(in.Image) == "" {
		return fmt.Errorf("image required")
	}
	if strings.TrimSpace(in.ExtendedRes) == "" {
		return fmt.Errorf("extended_resource or flavor_id required")
	}
	return nil
}
