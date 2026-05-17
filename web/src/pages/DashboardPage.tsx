import { useCallback, useEffect, useMemo, useState } from "react";
import * as api from "../api/endpoints";
import {
  firstMetricValue,
  parsePrometheusSamples,
  workloadsByPhase,
} from "../metrics/parsePrometheus";
import type { Allocation, LedgerSummary, WorkloadRecord } from "../types";

type AlertRule = {
  id: string;
  name: string;
  metric: string;
  threshold: number;
  op: "gt" | "gte";
  firing: boolean;
  current?: number;
};

const DEFAULT_ALERTS: AlertRule[] = [
  {
    id: "pending-pods",
    name: "集群 Pending Pod 过多",
    metric: "cp_k8s_pods_pending",
    threshold: 5,
    op: "gt",
    firing: false,
  },
  {
    id: "alloc-active",
    name: "活跃分配接近饱和",
    metric: "cp_allocations_active",
    threshold: 8,
    op: "gte",
    firing: false,
  },
];

function evalAlert(rule: AlertRule, value: number | undefined): AlertRule {
  if (value === undefined) return { ...rule, firing: false, current: undefined };
  const firing = rule.op === "gt" ? value > rule.threshold : value >= rule.threshold;
  return { ...rule, firing, current: value };
}

export function DashboardPage() {
  const [ledger, setLedger] = useState<LedgerSummary[]>([]);
  const [allocs, setAllocs] = useState<Allocation[]>([]);
  const [workloads, setWorkloads] = useState<WorkloadRecord[]>([]);
  const [metricsText, setMetricsText] = useState("");
  const [alerts, setAlerts] = useState<AlertRule[]>(DEFAULT_ALERTS);
  const [err, setErr] = useState<string | null>(null);

  const load = useCallback(async () => {
    setErr(null);
    try {
      const [l, a, w, m] = await Promise.all([
        api.ledgerSummary(),
        api.listAllocations(),
        api.listWorkloads(),
        api.fetchMetricsText(),
      ]);
      setLedger(l);
      setAllocs(a);
      setWorkloads(w.workloads || []);
      setMetricsText(m);

      const samples = parsePrometheusSamples(m);
      setAlerts(
        DEFAULT_ALERTS.map((r) => evalAlert(r, firstMetricValue(samples, r.metric))),
      );
      await api.observabilitySummary().catch(() => undefined);
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
    }
  }, []);

  useEffect(() => {
    void load();
    const t = setInterval(() => void load(), 20000);
    return () => clearInterval(t);
  }, [load]);

  const samples = useMemo(() => parsePrometheusSamples(metricsText), [metricsText]);
  const wlPhase = useMemo(() => workloadsByPhase(samples), [samples]);
  const pendingPods = firstMetricValue(samples, "cp_k8s_pods_pending");
  const activeAllocs = firstMetricValue(samples, "cp_allocations_active");

  const poolUtil = useMemo(() => {
    const byPool = new Map<string, { cap: number; used: number }>();
    for (const r of ledger) {
      const x = byPool.get(r.pool_id) || { cap: 0, used: 0 };
      x.cap += r.capacity_units;
      x.used += r.allocated_units;
      byPool.set(r.pool_id, x);
    }
    return [...byPool.entries()].map(([pool, v]) => ({
      pool,
      pct: v.cap > 0 ? Math.round((v.used / v.cap) * 100) : 0,
      used: v.used,
      cap: v.cap,
    }));
  }, [ledger]);

  const tenantTop = useMemo(() => {
    const m = new Map<string, number>();
    for (const a of allocs) {
      if (a.phase === "Released") continue;
      m.set(a.tenant, (m.get(a.tenant) || 0) + 1);
    }
    for (const w of workloads) {
      if (w.phase === "Running" || w.phase === "Pending") {
        const t = w.tenant || "_unknown";
        m.set(t, (m.get(t) || 0) + 1);
      }
    }
    return [...m.entries()]
      .sort((a, b) => b[1] - a[1])
      .slice(0, 8);
  }, [allocs, workloads]);

  return (
    <>
      <h1 className="page-title">集群监控</h1>
      <p className="page-desc">
        池利用率、租户消耗 Top、控制面 Prometheus 指标与静态阈值告警；eBPF 节点指标见 node-kernel-exporter。
      </p>

      <div className="row" style={{ marginBottom: "1rem" }}>
        <button type="button" className="btn ghost" onClick={() => void load()}>
          刷新
        </button>
        <a href="/metrics" target="_blank" rel="noreferrer" className="btn ghost">
          原始 /metrics
        </a>
      </div>

      {err && <div className="alert error">{err}</div>}

      <div className="grid2">
        <div className="card">
          <div className="stat-label">活跃分配 (cp_allocations_active)</div>
          <div className="stat">{activeAllocs ?? "—"}</div>
        </div>
        <div className="card">
          <div className="stat-label">Pending Pods (cp_k8s_pods_pending)</div>
          <div className="stat">{pendingPods ?? "—"}</div>
        </div>
        {Object.entries(wlPhase).map(([ph, n]) => (
          <div className="card" key={ph}>
            <div className="stat-label">任务 {ph}</div>
            <div className="stat">{n}</div>
          </div>
        ))}
      </div>

      <div className="grid2">
        <div className="card">
          <h3>资源池利用率</h3>
          {poolUtil.map((p) => (
            <div key={p.pool} style={{ marginBottom: "0.75rem" }}>
              <div className="row">
                <strong>{p.pool}</strong>
                <span className="muted small">
                  {p.used}/{p.cap} ({p.pct}%)
                </span>
              </div>
              <div className="progress">
                <i style={{ width: `${p.pct}%` }} />
              </div>
            </div>
          ))}
        </div>

        <div className="card">
          <h3>租户消耗 Top</h3>
          <table className="data">
            <thead>
              <tr>
                <th>租户</th>
                <th>活跃对象数</th>
              </tr>
            </thead>
            <tbody>
              {tenantTop.map(([t, n]) => (
                <tr key={t}>
                  <td>{t}</td>
                  <td>{n}</td>
                </tr>
              ))}
              {!tenantTop.length && (
                <tr>
                  <td colSpan={2} className="muted">
                    暂无数据
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </div>

      <div className="card">
        <h3>告警规则（静态阈值）</h3>
        <table className="data">
          <thead>
            <tr>
              <th>规则</th>
              <th>指标</th>
              <th>阈值</th>
              <th>当前值</th>
              <th>状态</th>
            </tr>
          </thead>
          <tbody>
            {alerts.map((a) => (
              <tr key={a.id}>
                <td>{a.name}</td>
                <td>
                  <code>{a.metric}</code>
                </td>
                <td>
                  {a.op === "gt" ? ">" : ">="} {a.threshold}
                </td>
                <td>{a.current ?? "—"}</td>
                <td>
                  {a.firing ? (
                    <span className="badge warn">FIRING</span>
                  ) : (
                    <span className="badge ok">OK</span>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="card">
        <h3>指标面板（Prometheus 样本摘要）</h3>
        <table className="data">
          <thead>
            <tr>
              <th>指标</th>
              <th>标签</th>
              <th>值</th>
            </tr>
          </thead>
          <tbody>
            {samples
              .filter((s) => s.name.startsWith("cp_"))
              .slice(0, 24)
              .map((s, i) => (
                <tr key={i}>
                  <td>
                    <code>{s.name}</code>
                  </td>
                  <td className="small muted">{JSON.stringify(s.labels)}</td>
                  <td>{s.value}</td>
                </tr>
              ))}
          </tbody>
        </table>
      </div>
    </>
  );
}
