/** 与控制面 OpenAPI v0 对齐的 TypeScript 类型（snake_case）。 */

export interface LoginResponse {
  access_token: string;
  token_type: string;
  expires_in: number;
  note?: string;
}

export interface MeResponse {
  sub: string;
  role: "admin" | "tenant";
  tenant: string;
}

export interface SliceFlavor {
  id: string;
  display_name: string;
  hardware_family: string;
  avi_template_id?: string;
  provision_mode?: string;
  virtualization_mode?: string;
  ai_cores: number;
  memory_mib: number;
  ai_cpus?: number;
  k8s_extended_resource: string;
  pod_limits_example: Record<string, string>;
  notes?: string;
}

export interface Pool {
  id: string;
  name: string;
  node_selector: Record<string, string>;
  supported_flavor_ids: string[];
}

export interface AcceleratorCard {
  id: string;
  node_name: string;
  chip_model: string;
  slot_index: number;
  physical_npus: number;
}

export interface SliceUnit {
  id: string;
  card_id: string;
  flavor_id: string;
  available: boolean;
}

export interface Node {
  name: string;
  labels: Record<string, string>;
  pool_id: string;
  cards: AcceleratorCard[];
  slices: SliceUnit[];
}

export interface LedgerSummary {
  pool_id: string;
  flavor_id: string;
  capacity_units: number;
  allocated_units: number;
  free_units: number;
}

export interface Allocation {
  id: string;
  tenant: string;
  pool_id: string;
  flavor_id: string;
  phase: string;
  slice_unit_id?: string;
  namespace?: string;
  pod_ref?: string;
  created_at: string;
  updated_at: string;
  annotations?: Record<string, string>;
}

export interface WorkloadRecord {
  id: string;
  tenant?: string;
  namespace: string;
  name: string;
  kind: "Training" | "Inference";
  phase: string;
  flavor_id?: string;
  extended_resource?: string;
  k8s_refs?: string[];
  pod_names?: string[];
  message?: string;
  events_summary?: string[];
  created_at: string;
  updated_at: string;
}

export interface K8sBinding {
  flavor_id: string;
  k8s_extended_resource: string;
  recommended_limits: Record<string, string>;
  recommended_requests: Record<string, string>;
  scheduling_hints?: Record<string, string>;
}

/** 与规划文档中「策略 API」字段对齐的聚合视图（当前由前端从多接口合成，便于日后切换为单端点）。 */
export interface PolicyCoscheduleResponse {
  generated_at: string;
  policy_version: string;
  conflicts: CoscheduleConflict[];
  queue_hints: CoscheduleQueueHint[];
  scheduling_context: {
    pending_by_tenant: Record<string, number>;
    running_training: number;
    running_inference: number;
    /** 与 Volcano priorityClass / queue 对齐的占位，待控制面下发 */
    default_priority_class?: string;
  };
}

export interface CoscheduleConflict {
  severity: "warn" | "info";
  code: string;
  message: string;
  related_workload_ids: string[];
  flavor_id?: string;
  pool_id?: string;
}

export interface CoscheduleQueueHint {
  flavor_id: string;
  pool_id: string;
  pending_workloads: number;
  free_units: number;
  capacity_units: number;
  suggested_action: string;
}

export interface TrainingWorkloadSpec {
  tenant?: string;
  allocation_id?: string;
  name: string;
  namespace: string;
  image: string;
  flavor_id?: string;
  extended_resource: string;
  resource_quantity?: string;
  replicas?: number;
  scheduler_name?: string;
  min_available?: number;
  task_name?: string;
  command?: string[];
  args?: string[];
  node_selector?: Record<string, string>;
  labels?: Record<string, string>;
  annotations?: Record<string, string>;
  distributed?: {
    enabled?: boolean;
    world_size?: number;
    master_addr?: string;
    master_port?: number;
    hccl_connect_timeout_sec?: number;
    device_backend?: string;
    extra_envs?: Record<string, string>;
  };
}

export interface InferenceWorkloadSpec {
  tenant?: string;
  allocation_id?: string;
  name: string;
  namespace: string;
  image: string;
  flavor_id?: string;
  extended_resource: string;
  resource_quantity?: string;
  replicas?: number;
  container_port?: number;
  service_type?: string;
  create_service?: boolean;
  command?: string[];
  args?: string[];
  node_selector?: Record<string, string>;
  labels?: Record<string, string>;
  annotations?: Record<string, string>;
}

export interface ObservabilitySummary {
  prometheus: Record<string, string>;
  gauges_hint?: string;
}
