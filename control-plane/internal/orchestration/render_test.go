package orchestration

import (
	"strings"
	"testing"
)

func TestRenderTrainingYAML_Single(t *testing.T) {
	yml, err := RenderTrainingYAML(TrainingWorkloadSpec{
		Name:        "demo-train",
		Namespace:   "default",
		Image:       "example.com/ascend-pytorch:test",
		ExtendedRes: "huawei.com/Ascend910B2",
		Replicas:    1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(yml, "batch.volcano.sh/v1alpha1") {
		t.Fatalf("expected volcano job, got:\n%s", yml)
	}
	if !strings.Contains(yml, "huawei.com/Ascend910B2") {
		t.Fatalf("missing extended resource in yaml:\n%s", yml)
	}
}

func TestRenderTrainingYAML_Distributed(t *testing.T) {
	yml, err := RenderTrainingYAML(TrainingWorkloadSpec{
		Name:        "dist-train",
		Namespace:   "tenant-a",
		Image:       "example.com/ascend-pytorch:test",
		ExtendedRes: "huawei.com/Ascend910B2",
		Replicas:    4,
		Distributed: DistributedParams{
			Enabled:           true,
			WorldSize:         4,
			MasterPort:        29500,
			RankFromPodName:   true,
			ExtraEnvs: map[string]string{
				"TORCH_DISTRIBUTED_DEBUG": "DETAIL",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(yml, "\n---\n") {
		t.Fatalf("expected multi-doc yaml with master service:\n%s", yml)
	}
	if !strings.Contains(yml, "kind: Service") || !strings.Contains(yml, "clusterIP: None") {
		t.Fatalf("expected headless service:\n%s", yml)
	}
	wantMaster := "dist-train-master.tenant-a.svc.cluster.local"
	if !strings.Contains(yml, wantMaster) {
		t.Fatalf("expected default MASTER_ADDR %q in yaml:\n%s", wantMaster, yml)
	}
	for _, token := range []string{"MASTER_ADDR", "HCCL_IF_IP", "WORLD_SIZE", "TORCH_DISTRIBUTED_DEBUG", "POD_NAME", "POD_NAME##*-", "LOCAL_RANK=0"} {
		if !strings.Contains(yml, token) {
			t.Fatalf("expected %q in yaml:\n%s", token, yml)
		}
	}
}

func TestRenderTrainingYAML_MultiReplicaRequiresRankFromPodName(t *testing.T) {
	_, err := RenderTrainingYAML(TrainingWorkloadSpec{
		Name:        "bad",
		Namespace:   "default",
		Image:       "x",
		ExtendedRes: "huawei.com/Ascend910B2",
		Replicas:    2,
		Distributed: DistributedParams{
			Enabled:         true,
			RankFromPodName: false,
		},
	})
	if err == nil {
		t.Fatal("expected error when replicas>1 without rank_from_pod_name")
	}
}

func TestRenderInferenceYAML_NoService(t *testing.T) {
	yml, err := RenderInferenceYAML(InferenceWorkloadSpec{
		Name:          "demo-infer",
		Namespace:     "default",
		Image:         "example.com/ascend-mindspore:test",
		ExtendedRes:   "huawei.com/ascend-310-2c",
		Replicas:      2,
		CreateService: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(yml, "\n---\n") {
		t.Fatalf("unexpected multi-doc when service disabled:\n%s", yml)
	}
	if !strings.Contains(yml, "kind: Deployment") {
		t.Fatalf("expected deployment:\n%s", yml)
	}
}

func TestRenderInferenceYAML_WithService(t *testing.T) {
	yml, err := RenderInferenceYAML(InferenceWorkloadSpec{
		Name:          "demo-infer",
		Namespace:     "default",
		Image:         "example.com/ascend-mindspore:test",
		ExtendedRes:   "huawei.com/ascend-310-2c",
		Replicas:      1,
		CreateService: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(yml, "kind: Service") {
		t.Fatalf("expected service:\n%s", yml)
	}
}
