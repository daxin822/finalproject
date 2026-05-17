package orchestration

import (
	"context"
	"strings"
	"time"

	"finalproject/control-plane/internal/model"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// StatusSyncer 轮询 Volcano Job / Deployment 与 Pod，回写 WorkloadStore。
type StatusSyncer struct {
	Cluster  *Cluster
	Store    *WorkloadStore
	Interval time.Duration
}

func NewStatusSyncer(c *Cluster, store *WorkloadStore, interval time.Duration) *StatusSyncer {
	if interval <= 0 {
		interval = 10 * time.Second
	}
	return &StatusSyncer{Cluster: c, Store: store, Interval: interval}
}

func (s *StatusSyncer) Run(ctx context.Context) {
	if s == nil || s.Cluster == nil || !s.Cluster.Enabled() || s.Store == nil {
		return
	}
	t := time.NewTicker(s.Interval)
	defer t.Stop()
	s.syncAll(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.syncAll(ctx)
		}
	}
}

func (s *StatusSyncer) syncAll(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	for _, rec := range s.Store.List("", "") {
		switch rec.Kind {
		case model.WorkloadKindTraining:
			s.syncTraining(ctx, rec)
		case model.WorkloadKindInference:
			s.syncInference(ctx, rec)
		}
	}
}

func volcanoJobGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "batch.volcano.sh", Version: "v1alpha1", Resource: "jobs"}
}

func deploymentGVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
}

func (s *StatusSyncer) syncTraining(ctx context.Context, rec *model.WorkloadRecord) {
	jobRef, ok := findRef(rec.K8sRefs, "Job", "batch.volcano.sh")
	if !ok {
		return
	}
	_, _, _, ns, name, ok := ParseWorkloadRef(jobRef)
	if !ok || ns == "" || name == "" {
		return
	}
	obj, err := s.Cluster.GetUnstructured(ctx, volcanoJobGVR(), ns, name)
	jobState := ""
	if err != nil {
		if errors.IsNotFound(err) {
			s.Store.Update(rec.ID, func(r *model.WorkloadRecord) {
				r.Phase = model.WorkloadPhaseFailed
				r.Message = "volcano job not found (deleted?)"
			})
			return
		}
		return
	}
	jobState, _, _ = unstructured.NestedString(obj.Object, "status", "state")
	if jobState == "" {
		jobState, _, _ = unstructured.NestedString(obj.Object, "status", "phase")
	}
	pods, err := s.Cluster.ListPods(ctx, ns, "control-plane.finalproject/workload-name="+rec.Name)
	if err != nil {
		return
	}
	podNames := make([]string, 0, len(pods))
	for _, p := range pods {
		podNames = append(podNames, p.Name)
	}
	phase, msg := MapTrainingPhase(jobState, pods)
	s.Store.Update(rec.ID, func(r *model.WorkloadRecord) {
		r.Phase = phase
		if msg != "" {
			r.Message = msg
		}
		r.PodNames = podNames
		if phase == model.WorkloadPhaseFailed || phase == model.WorkloadPhaseUnknown {
			if ev, eerr := s.Cluster.FetchEventsSummary(ctx, ns, name, 5); eerr == nil {
				r.EventsSummary = ev
			}
		} else if phase == model.WorkloadPhasePending && len(podNames) == 0 {
			if ev, eerr := s.Cluster.FetchEventsSummary(ctx, ns, name, 5); eerr == nil {
				r.EventsSummary = ev
			}
		}
	})
}

func (s *StatusSyncer) syncInference(ctx context.Context, rec *model.WorkloadRecord) {
	depRef, ok := findRef(rec.K8sRefs, "Deployment", "apps")
	if !ok {
		return
	}
	_, _, _, ns, name, ok := ParseWorkloadRef(depRef)
	if !ok || ns == "" || name == "" {
		return
	}
	obj, err := s.Cluster.GetUnstructured(ctx, deploymentGVR(), ns, name)
	if err != nil {
		if errors.IsNotFound(err) {
			s.Store.Update(rec.ID, func(r *model.WorkloadRecord) {
				r.Phase = model.WorkloadPhaseFailed
				r.Message = "deployment not found"
			})
			return
		}
		return
	}
	avail, foundAvail, err := unstructured.NestedInt64(obj.Object, "status", "availableReplicas")
	if err != nil {
		return
	}
	if !foundAvail {
		avail = 0
	}
	desired, foundDesired, err := unstructured.NestedInt64(obj.Object, "spec", "replicas")
	if err != nil {
		return
	}
	if !foundDesired || desired <= 0 {
		desired = 1
	}
	pods, err := s.Cluster.ListPods(ctx, ns, "app.kubernetes.io/name="+rec.Name)
	if err != nil {
		return
	}
	podNames := make([]string, 0, len(pods))
	for _, p := range pods {
		podNames = append(podNames, p.Name)
	}
	phase := MapInferencePhase(int32(avail), int32(desired), pods)
	s.Store.Update(rec.ID, func(r *model.WorkloadRecord) {
		r.Phase = phase
		r.PodNames = podNames
		if phase == model.WorkloadPhaseFailed {
			if ev, eerr := s.Cluster.FetchEventsSummary(ctx, ns, name, 5); eerr == nil {
				r.EventsSummary = ev
			}
		}
	})
}

func findRef(refs []string, kind, group string) (string, bool) {
	for _, r := range refs {
		g, _, k, _, _, ok := ParseWorkloadRef(r)
		if !ok {
			continue
		}
		if strings.EqualFold(k, kind) && g == group {
			return r, true
		}
	}
	return "", false
}
