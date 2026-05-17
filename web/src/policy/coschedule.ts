import type {
  Allocation,
  CoscheduleConflict,
  CoscheduleQueueHint,
  LedgerSummary,
  Node,
  PolicyCoscheduleResponse,
  WorkloadRecord,
} from "../types";

function flavorToPools(nodes: Node[], flavorId: string): Set<string> {
  const pools = new Set<string>();
  for (const n of nodes) {
    for (const s of n.slices) {
      if (s.flavor_id === flavorId) pools.add(n.pool_id);
    }
  }
  return pools;
}

function ledgerRow(
  ledger: LedgerSummary[],
  poolId: string,
  flavorId: string,
): LedgerSummary | undefined {
  return ledger.find((r) => r.pool_id === poolId && r.flavor_id === flavorId);
}

/**
 * 从现有只读接口合成训推协同/冲突视图；字段命名与 `PolicyCoscheduleResponse` 契约一致，
 * 后续可将实现替换为 `GET /api/v1/policy/coschedule` 的 JSON 反序列化。
 */
export function buildPolicyCoscheduleResponse(
  allocations: Allocation[],
  workloads: WorkloadRecord[],
  ledger: LedgerSummary[],
  nodes: Node[],
): PolicyCoscheduleResponse {
  const activeAllocs = allocations.filter((a) => a.phase !== "Released");
  const pendingByTenant: Record<string, number> = {};
  let runningTraining = 0;
  let runningInference = 0;

  for (const w of workloads) {
    if (w.phase === "Pending") {
      const t = w.tenant || "_unknown";
      pendingByTenant[t] = (pendingByTenant[t] || 0) + 1;
    }
    if (w.phase === "Running") {
      if (w.kind === "Training") runningTraining++;
      if (w.kind === "Inference") runningInference++;
    }
  }

  const conflicts: CoscheduleConflict[] = [];

  const flavorPending = new Map<string, WorkloadRecord[]>();
  for (const w of workloads) {
    if (w.phase !== "Pending" || !w.flavor_id) continue;
    const arr = flavorPending.get(w.flavor_id) || [];
    arr.push(w);
    flavorPending.set(w.flavor_id, arr);
  }

  for (const [fid, arr] of flavorPending) {
    if (arr.length <= 1) continue;
    conflicts.push({
      severity: "warn",
      code: "QUEUE_DEPTH",
      message: `规格 ${fid} 上存在 ${arr.length} 个 Pending 任务，可能存在排队与资源争用。`,
      related_workload_ids: arr.map((x) => x.id),
      flavor_id: fid,
    });
  }

  for (const w of workloads) {
    if (w.phase !== "Pending" || !w.flavor_id) continue;
    const pools = flavorToPools(nodes, w.flavor_id);
    for (const poolId of pools) {
      const row = ledgerRow(ledger, poolId, w.flavor_id);
      if (row && row.free_units === 0) {
        conflicts.push({
          severity: "warn",
          code: "NO_FREE_SLICE",
          message: `池 ${poolId} / 规格 ${w.flavor_id} 当前无空闲切片，任务 ${w.name} 可能长时间 Pending。`,
          related_workload_ids: [w.id],
          flavor_id: w.flavor_id,
          pool_id: poolId,
        });
      }
    }
  }

  const allocBySlice = new Map<string, Allocation[]>();
  for (const a of activeAllocs) {
    if (!a.slice_unit_id) continue;
    const list = allocBySlice.get(a.slice_unit_id) || [];
    list.push(a);
    allocBySlice.set(a.slice_unit_id, list);
  }
  for (const [sid, list] of allocBySlice) {
    if (list.length > 1) {
      conflicts.push({
        severity: "info",
        code: "MULTI_ALLOC_SAME_SLICE",
        message: `切片单元 ${sid} 上存在多条未释放分配记录（演示数据或异常），请核对账本。`,
        related_workload_ids: [],
      });
    }
  }

  const queue_hints: CoscheduleQueueHint[] = [];
  for (const row of ledger) {
    const pend = workloads.filter(
      (w) =>
        w.phase === "Pending" &&
        w.flavor_id === row.flavor_id &&
        flavorToPools(nodes, row.flavor_id).has(row.pool_id),
    );
    if (pend.length === 0 && row.free_units > 0) continue;
    let action = "资源充足，可按优先级调度。";
    if (row.free_units === 0) action = "无空闲切片：建议扩容、释放分配或排队等待。";
    else if (pend.length > row.free_units)
      action = `排队任务多于空闲切片（${pend.length} > ${row.free_units}），建议调整优先级或 SLA。`;
    else if (pend.length > 0) action = "存在排队任务，请关注 Volcano 队列与抢占策略。";

    queue_hints.push({
      flavor_id: row.flavor_id,
      pool_id: row.pool_id,
      pending_workloads: pend.length,
      free_units: row.free_units,
      capacity_units: row.capacity_units,
      suggested_action: action,
    });
  }

  return {
    generated_at: new Date().toISOString(),
    policy_version: "v0-frontend-synthetic",
    conflicts,
    queue_hints,
    scheduling_context: {
      pending_by_tenant: pendingByTenant,
      running_training: runningTraining,
      running_inference: runningInference,
      default_priority_class: "待控制面下发 priorityClassName",
    },
  };
}
