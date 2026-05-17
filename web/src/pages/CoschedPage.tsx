import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import * as api from "../api/endpoints";
import { buildPolicyCoscheduleResponse } from "../policy/coschedule";
import type { PolicyCoscheduleResponse } from "../types";

export function CoschedPage() {
  const [policy, setPolicy] = useState<PolicyCoscheduleResponse | null>(null);
  const [err, setErr] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    setLoading(true);
    setErr(null);
    try {
      const [allocs, wl, ledger, nodes] = await Promise.all([
        api.listAllocations(),
        api.listWorkloads(),
        api.ledgerSummary(),
        api.listNodes(),
      ]);
      setPolicy(
        buildPolicyCoscheduleResponse(
          allocs,
          wl.workloads || [],
          ledger,
          nodes,
        ),
      );
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
    const t = setInterval(() => void load(), 15000);
    return () => clearInterval(t);
  }, [load]);

  const ctx = policy?.scheduling_context;

  return (
    <>
      <h1 className="page-title">训推协同</h1>
      <p className="page-desc">
        只读冲突面板：同规格排队深度、空闲切片与 Pending 任务对比；字段与策略 API 契约{" "}
        <code>PolicyCoscheduleResponse</code> 对齐（当前由多接口合成，可平滑切换后端单端点）。
      </p>

      <div className="row" style={{ marginBottom: "1rem" }}>
        <button type="button" className="btn ghost" onClick={() => void load()} disabled={loading}>
          刷新
        </button>
        <Link to="/jobs" className="btn ghost">
          前往任务列表
        </Link>
        {policy && (
          <span className="muted small">
            生成于 {new Date(policy.generated_at).toLocaleString()} · {policy.policy_version}
          </span>
        )}
      </div>

      {err && <div className="alert error">{err}</div>}

      {ctx && (
        <div className="grid2">
          <div className="card">
            <div className="stat-label">运行中训练</div>
            <div className="stat">{ctx.running_training}</div>
          </div>
          <div className="card">
            <div className="stat-label">运行中推理</div>
            <div className="stat">{ctx.running_inference}</div>
          </div>
          <div className="card">
            <div className="stat-label">默认优先级类</div>
            <div className="small">{ctx.default_priority_class || "—"}</div>
          </div>
          <div className="card">
            <div className="stat-label">按租户 Pending</div>
            <ul className="small">
              {Object.entries(ctx.pending_by_tenant).map(([t, n]) => (
                <li key={t}>
                  {t}: {n}
                </li>
              ))}
              {Object.keys(ctx.pending_by_tenant).length === 0 && (
                <li className="muted">无排队任务</li>
              )}
            </ul>
          </div>
        </div>
      )}

      <div className="card">
        <h3>冲突与提示</h3>
        {!policy?.conflicts.length && !loading && (
          <p className="muted">当前未检测到资源冲突或排队风险。</p>
        )}
        <table className="data">
          <thead>
            <tr>
              <th>级别</th>
              <th>代码</th>
              <th>说明</th>
              <th>关联任务</th>
            </tr>
          </thead>
          <tbody>
            {(policy?.conflicts || []).map((c, i) => (
              <tr key={i}>
                <td>
                  <span className={c.severity === "warn" ? "badge warn" : "badge"}>
                    {c.severity}
                  </span>
                </td>
                <td>
                  <code>{c.code}</code>
                </td>
                <td>{c.message}</td>
                <td className="small">
                  {c.related_workload_ids.length
                    ? c.related_workload_ids.join(", ")
                    : "—"}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="card">
        <h3>排队 / 容量建议</h3>
        <table className="data">
          <thead>
            <tr>
              <th>池</th>
              <th>规格</th>
              <th>Pending</th>
              <th>空闲 / 容量</th>
              <th>建议</th>
            </tr>
          </thead>
          <tbody>
            {(policy?.queue_hints || []).map((h, i) => (
              <tr key={i}>
                <td>{h.pool_id}</td>
                <td className="small">{h.flavor_id}</td>
                <td>{h.pending_workloads}</td>
                <td>
                  {h.free_units} / {h.capacity_units}
                </td>
                <td>{h.suggested_action}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </>
  );
}
