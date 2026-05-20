package orchestration

// TrainingWorkloadSpec 描述 MindCluster/Volcano 路径下的训练任务输入（控制面 → YAML）。
type TrainingWorkloadSpec struct {
	Tenant      string            `json:"tenant,omitempty"`
	AllocationID string           `json:"allocation_id,omitempty"`
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	Image       string            `json:"image"`
	Command     []string          `json:"command,omitempty"`
	Args        []string          `json:"args,omitempty"`
	FlavorID    string            `json:"flavor_id,omitempty"`
	PoolLabel   string            `json:"pool_label,omitempty"`
	ExtendedRes       string            `json:"extended_resource"`
	ResQuantity       string            `json:"resource_quantity"`
	ExtendedResMemory string            `json:"extended_memory_resource,omitempty"`
	MemoryQuantity    string            `json:"memory_quantity,omitempty"`
	Replicas          int               `json:"replicas"`
	Scheduler   string            `json:"scheduler_name"`
	MinAvailable int              `json:"min_available"`
	TaskName    string            `json:"task_name"`
	NodeSelector map[string]string `json:"node_selector,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Distributed DistributedParams `json:"distributed,omitempty"`
}

// DistributedParams 昇腾分布式训练常见集合通信 / 框架侧参数（与 MindCluster 文档对齐时在此扩展）。
type DistributedParams struct {
	Enabled             bool              `json:"enabled"`
	WorldSize           int               `json:"world_size"`
	MasterAddr          string            `json:"master_addr,omitempty"`
	MasterPort          int               `json:"master_port,omitempty"`
	CreateMasterService *bool             `json:"create_master_service,omitempty"`
	RankFromPodName     bool              `json:"rank_from_pod_name,omitempty"`
	HCCLTimeout         int               `json:"hccl_connect_timeout_sec,omitempty"`
	DeviceBackend       string            `json:"device_backend,omitempty"`
	ExtraEnvs           map[string]string `json:"extra_envs,omitempty"`
}

// InferenceWorkloadSpec 推理 Deployment 模板输入。
type InferenceWorkloadSpec struct {
	Tenant           string            `json:"tenant,omitempty"`
	AllocationID     string            `json:"allocation_id,omitempty"`
	Name             string            `json:"name"`
	Namespace        string            `json:"namespace"`
	Image            string            `json:"image"`
	FlavorID         string            `json:"flavor_id,omitempty"`
	Command          []string          `json:"command,omitempty"`
	Args             []string          `json:"args,omitempty"`
	ExtendedRes       string            `json:"extended_resource"`
	ResQuantity       string            `json:"resource_quantity"`
	ExtendedResMemory string            `json:"extended_memory_resource,omitempty"`
	MemoryQuantity    string            `json:"memory_quantity,omitempty"`
	Replicas          int               `json:"replicas"`
	Port             int               `json:"container_port"`
	ServiceType      string            `json:"service_type"`
	CreateService    bool              `json:"create_service"`
	NodeSelector     map[string]string `json:"node_selector,omitempty"`
	Labels           map[string]string `json:"labels,omitempty"`
	Annotations      map[string]string `json:"annotations,omitempty"`
	ResourceRequests map[string]string `json:"resource_requests,omitempty"`
}
