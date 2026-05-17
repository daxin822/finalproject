package orchestration

import (
	"testing"

	"finalproject/control-plane/internal/model"
)

func TestWorkloadStoreCreateGetList(t *testing.T) {
	s := NewWorkloadStore()
	s.Create(&model.WorkloadRecord{
		ID:        "a1",
		Tenant:    "t1",
		Namespace: "ns",
		Name:      "job1",
		Kind:      model.WorkloadKindTraining,
		Phase:     model.WorkloadPhasePending,
	})
	if _, ok := s.Get("a1"); !ok {
		t.Fatal("expected record")
	}
	all := s.List("", "")
	if len(all) != 1 {
		t.Fatalf("list all: got %d", len(all))
	}
	byTenant := s.List("t1", "")
	if len(byTenant) != 1 {
		t.Fatalf("list tenant: got %d", len(byTenant))
	}
	byPhase := s.List("", model.WorkloadPhasePending)
	if len(byPhase) != 1 {
		t.Fatalf("list phase: got %d", len(byPhase))
	}
}

func TestWorkloadStoreUpdatePhase(t *testing.T) {
	s := NewWorkloadStore()
	s.Create(&model.WorkloadRecord{ID: "x", Namespace: "n", Name: "j", Kind: model.WorkloadKindTraining, Phase: model.WorkloadPhasePending})
	if !s.UpdatePhase("x", model.WorkloadPhaseRunning, "") {
		t.Fatal("update phase")
	}
	rec, _ := s.Get("x")
	if rec.Phase != model.WorkloadPhaseRunning {
		t.Fatalf("phase=%s", rec.Phase)
	}
}

func TestMapTrainingPhaseTransitions(t *testing.T) {
	pendingPods := []PodSummary{{Name: "p1", Phase: "Pending"}}
	ph, _ := MapTrainingPhase("", pendingPods)
	if ph != model.WorkloadPhasePending {
		t.Fatalf("expected Pending, got %s", ph)
	}
	ph2, _ := MapTrainingPhase("Running", []PodSummary{{Name: "p1", Phase: "Running"}})
	if ph2 != model.WorkloadPhaseRunning {
		t.Fatalf("expected Running, got %s", ph2)
	}
	ph3, _ := MapTrainingPhase("Completed", nil)
	if ph3 != model.WorkloadPhaseSucceeded {
		t.Fatalf("expected Succeeded, got %s", ph3)
	}
}

func TestParseWorkloadRef(t *testing.T) {
	g, v, k, ns, n, ok := ParseWorkloadRef("batch.volcano.sh/v1alpha1/Job/default/myjob")
	if !ok || g != "batch.volcano.sh" || v != "v1alpha1" || k != "Job" || ns != "default" || n != "myjob" {
		t.Fatalf("parse volcano: %v %v %v %v %v %v", g, v, k, ns, n, ok)
	}
	g2, v2, k2, ns2, n2, ok2 := ParseWorkloadRef("/v1/Service/default/svc")
	if !ok2 || g2 != "" || v2 != "v1" || k2 != "Service" || ns2 != "default" || n2 != "svc" {
		t.Fatalf("parse core svc: %v %v %v %v %v %v", g2, v2, k2, ns2, n2, ok2)
	}
}
