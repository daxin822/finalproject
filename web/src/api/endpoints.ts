import { apiFetch, newIdempotencyKey } from "./client";
import type {
  Allocation,
  InferenceWorkloadSpec,
  K8sBinding,
  LedgerSummary,
  LoginResponse,
  MeResponse,
  Node,
  ObservabilitySummary,
  Pool,
  SliceFlavor,
  TrainingWorkloadSpec,
  WorkloadRecord,
} from "../types";

export async function login(username: string, password: string) {
  return apiFetch<LoginResponse>("/api/v1/auth/login", {
    method: "POST",
    json: { username, password },
  });
}

export async function me() {
  return apiFetch<MeResponse>("/api/v1/auth/me");
}

export async function listFlavors() {
  return apiFetch<SliceFlavor[]>("/api/v1/catalog/flavors");
}

export async function listPools() {
  return apiFetch<Pool[]>("/api/v1/pools");
}

export async function listNodes() {
  return apiFetch<Node[]>("/api/v1/inventory/nodes");
}

export async function ledgerSummary() {
  return apiFetch<LedgerSummary[]>("/api/v1/ledger/summary");
}

export async function listAllocations() {
  return apiFetch<Allocation[]>("/api/v1/allocations");
}

export async function createAllocation(body: {
  tenant: string;
  pool_id: string;
  flavor_id: string;
  namespace?: string;
  pod_ref?: string;
}) {
  return apiFetch<{ allocation: Allocation; k8s: Record<string, unknown> }>(
    "/api/v1/allocations",
    {
      method: "POST",
      json: body,
      headers: { "Idempotency-Key": newIdempotencyKey() },
    },
  );
}

export async function releaseAllocation(id: string) {
  return apiFetch<Allocation>(`/api/v1/allocations/${id}/release`, {
    method: "POST",
  });
}

export async function k8sBinding(flavorId: string) {
  return apiFetch<K8sBinding>(`/api/v1/k8s/bindings/${encodeURIComponent(flavorId)}`);
}

export async function listWorkloads(qs?: { tenant?: string; phase?: string }) {
  const p = new URLSearchParams();
  if (qs?.tenant) p.set("tenant", qs.tenant);
  if (qs?.phase) p.set("phase", qs.phase);
  const q = p.toString();
  return apiFetch<{ workloads: WorkloadRecord[] }>(
    `/api/v1/workloads${q ? `?${q}` : ""}`,
  );
}

export async function getWorkload(id: string) {
  return apiFetch<WorkloadRecord>(`/api/v1/workloads/${encodeURIComponent(id)}`);
}

export async function renderTraining(spec: TrainingWorkloadSpec) {
  return apiFetch<{ yaml: string; spec: TrainingWorkloadSpec }>(
    "/api/v1/workloads/training/render",
    { method: "POST", json: spec },
  );
}

export async function submitTraining(spec: TrainingWorkloadSpec) {
  return apiFetch<{ job_id: string; phase: string; workload: WorkloadRecord }>(
    "/api/v1/workloads/training",
    {
      method: "POST",
      json: spec,
      headers: { "Idempotency-Key": newIdempotencyKey() },
    },
  );
}

export async function renderInference(spec: InferenceWorkloadSpec) {
  return apiFetch<{ yaml: string; spec: InferenceWorkloadSpec }>(
    "/api/v1/workloads/inference/render",
    { method: "POST", json: spec },
  );
}

export async function submitInference(spec: InferenceWorkloadSpec) {
  return apiFetch<{ service_id: string; workload: WorkloadRecord }>(
    "/api/v1/workloads/inference",
    {
      method: "POST",
      json: spec,
      headers: { "Idempotency-Key": newIdempotencyKey() },
    },
  );
}

export async function deleteWorkload(id: string) {
  return apiFetch<{ deleted: boolean; id: string }>(
    `/api/v1/workloads/${encodeURIComponent(id)}`,
    { method: "DELETE" },
  );
}

export async function fetchMetricsText() {
  return apiFetch<string>("/metrics", { headers: { Accept: "text/plain" } });
}

export async function observabilitySummary() {
  return apiFetch<ObservabilitySummary>("/api/v1/observability/summary");
}
