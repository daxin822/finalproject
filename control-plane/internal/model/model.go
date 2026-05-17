package model

import "time"

type AllocationPhase string

const (
	PhaseRequested AllocationPhase = "Requested"
	PhaseBound     AllocationPhase = "Bound"
	PhaseInUse     AllocationPhase = "InUse"
	PhaseReleased  AllocationPhase = "Released"
	PhaseFailed    AllocationPhase = "Failed"
)

type AcceleratorCard struct {
	ID           string `json:"id"`
	NodeName     string `json:"node_name"`
	ChipModel    string `json:"chip_model"`
	SlotIndex    int    `json:"slot_index"`
	PhysicalNPUs int    `json:"physical_npus"`
}

type SliceUnit struct {
	ID        string `json:"id"`
	CardID    string `json:"card_id"`
	FlavorID  string `json:"flavor_id"`
	Available bool   `json:"available"`
}

type Node struct {
	Name     string            `json:"name"`
	Labels   map[string]string `json:"labels"`
	PoolID   string            `json:"pool_id"`
	Cards    []AcceleratorCard `json:"cards"`
	Slices   []SliceUnit       `json:"slices"`
}

type Allocation struct {
	ID          string            `json:"id"`
	Tenant      string            `json:"tenant"`
	PoolID      string            `json:"pool_id"`
	FlavorID    string            `json:"flavor_id"`
	Phase       AllocationPhase   `json:"phase"`
	SliceUnitID string            `json:"slice_unit_id,omitempty"`
	PodRef      string            `json:"pod_ref,omitempty"`
	Namespace   string            `json:"namespace,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

type LedgerSummary struct {
	PoolID         string         `json:"pool_id"`
	FlavorID       string         `json:"flavor_id"`
	CapacityUnits  int            `json:"capacity_units"`
	AllocatedUnits int            `json:"allocated_units"`
	FreeUnits      int            `json:"free_units"`
}

type K8sResourceBinding struct {
	FlavorID              string            `json:"flavor_id"`
	K8sExtendedResource   string            `json:"k8s_extended_resource"`
	RecommendedLimits     map[string]string `json:"recommended_limits"`
	RecommendedRequests   map[string]string `json:"recommended_requests"`
	SchedulingHints         map[string]string `json:"scheduling_hints,omitempty"`
}
