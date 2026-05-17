package model

import "time"

// WorkloadKind 区分训练（Volcano Job）与推理（Deployment）。
type WorkloadKind string

const (
	WorkloadKindTraining  WorkloadKind = "Training"
	WorkloadKindInference WorkloadKind = "Inference"
)

// WorkloadPhase 与控制面任务生命周期对齐。
type WorkloadPhase string

const (
	WorkloadPhasePending   WorkloadPhase = "Pending"
	WorkloadPhaseRunning   WorkloadPhase = "Running"
	WorkloadPhaseSucceeded WorkloadPhase = "Succeeded"
	WorkloadPhaseFailed    WorkloadPhase = "Failed"
	WorkloadPhaseUnknown   WorkloadPhase = "Unknown"
)

// WorkloadRecord 为业务侧任务快照（内存 Store 持久化单元）。
type WorkloadRecord struct {
	ID               string         `json:"id"`
	Tenant           string         `json:"tenant,omitempty"`
	Namespace        string         `json:"namespace"`
	Name             string         `json:"name"`
	Kind             WorkloadKind   `json:"kind"`
	Phase            WorkloadPhase  `json:"phase"`
	FlavorID         string         `json:"flavor_id,omitempty"`
	ExtendedResource string         `json:"extended_resource,omitempty"`
	K8sRefs          []string       `json:"k8s_refs,omitempty"`
	PodNames         []string       `json:"pod_names,omitempty"`
	Message          string         `json:"message,omitempty"`
	EventsSummary    []string       `json:"events_summary,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}
