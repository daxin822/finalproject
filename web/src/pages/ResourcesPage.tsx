import { useCallback, useEffect, useMemo, useState } from "react";
import * as api from "../api/endpoints";
import { useAuth } from "../auth/AuthContext";
import type {
  Allocation,
  LedgerSummary,
  Node,
  Pool,
  SliceFlavor,
} from "../types";

type Tab = "templates" | "topology" | "quota" | "mine";

function phaseBadge(phase: string) {
  if (phase === "Released") return "badge";
  if (phase === "Failed") return "badge warn";
  return "badge ok";
}

export function ResourcesPage() {
  const { auth } = useAuth();
  const tenant =
    auth.status === "ready" && auth.me.role === "tenant" ? auth.me.tenant : "";

  const [tab, setTab] = useState<Tab>("templates");
  const [flavors, setFlavors] = useState<SliceFlavor[]>([]);
  const [pools, setPools] = useState<Pool[]>([]);
  const [nodes, setNodes] = useState<Node[]>([]);
  const [ledger, setLedger] = useState<LedgerSummary[]>([]);
  const [allocs, setAllocs] = useState<Allocation[]>([]);
  const [err, setErr] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const [filterPool, setFilterPool] = useState("");
  const [filterTenant, setFilterTenant] = useState("");

  const [wizardOpen, setWizardOpen] = useState(false);
  const [wizPool, setWizPool] = useState("");
  const [wizFlavor, setWizFlavor] = useState("");
  const [wizNs, setWizNs] = useState("default");
  const [preview, setPreview] = useState<string | null>(null);

  const load = useCallback(async () => {
    setErr(null);
    try {
      const [f, p, n, l, a] = await Promise.all([
        api.listFlavors(),
        api.listPools(),
        api.listNodes(),
        api.ledgerSummary(),
        api.listAllocations(),
      ]);
      setFlavors(f);
      setPools(p);
      setNodes(n);
      setLedger(l);
      setAllocs(a);
      if (!wizPool && p[0]) setWizPool(p[0].id);
      if (!wizFlavor && f[0]) setWizFlavor(f[0].id);
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
    }
  }, [wizPool, wizFlavor]);

  useEffect(() => {
    void load();
  }, [load]);

  const poolMap = useMemo(() => {
    const m = new Map<string, Pool>();
    pools.forEach((p) => m.set(p.id, p));
    return m;
  }, [pools]);

  const flavorMap = useMemo(() => {
    const m = new Map<string, SliceFlavor>();
    flavors.forEach((f) => m.set(f.id, f));
    return m;
  }, [flavors]);

  const filteredAllocs = useMemo(() => {
    return allocs.filter((a) => {
      if (filterPool && a.pool_id !== filterPool) return false;
      if (filterTenant && a.tenant !== filterTenant) return false;
      return true;
    });
  }, [allocs, filterPool, filterTenant]);

  const tenants = useMemo(() => {
    const s = new Set(allocs.map((a) => a.tenant));
    return [...s].sort();
  }, [allocs]);

  const supportedFlavors = useMemo(() => {
    const p = pools.find((x) => x.id === wizPool);
    if (!p) return flavors;
    return flavors.filter((f) => p.supported_flavor_ids.includes(f.id));
  }, [pools, wizPool, flavors]);

  async function previewBinding() {
    if (!wizFlavor) return;
    try {
      const b = await api.k8sBinding(wizFlavor);
      setPreview(JSON.stringify(b, null, 2));
    } catch (e) {
      setPreview(String(e instanceof Error ? e.message : e));
    }
  }

  async function submitAlloc() {
    setBusy(true);
    setErr(null);
    try {
      const t =
        tenant ||
        filterTenant ||
        (auth.status === "ready" ? auth.me.tenant || "tenant1" : "tenant1");
      await api.createAllocation({
        tenant: t,
        pool_id: wizPool,
        flavor_id: wizFlavor,
        namespace: wizNs,
      });
      setWizardOpen(false);
      await load();
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  }

  async function release(id: string) {
    if (!confirm(`释放分配 ${id}？`)) return;
    setBusy(true);
    try {
      await api.releaseAllocation(id);
      await load();
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  }

  const nodesByPool = useMemo(() => {
    const m = new Map<string, Node[]>();
    for (const n of nodes) {
      if (filterPool && n.pool_id !== filterPool) continue;
      const arr = m.get(n.pool_id) || [];
      arr.push(n);
      m.set(n.pool_id, arr);
    }
    return m;
  }, [nodes, filterPool]);

  return (
    <>
      <h1 className="page-title">虚拟化切片与资源池</h1>
      <p className="page-desc">
        管理 AVI 切片模板、池→节点→切片拓扑，以及租户配额与分配生命周期（Requested → Bound → InUse →
        Released）。
      </p>

      <div className="row" style={{ marginBottom: "1rem" }}>
        <select value={filterPool} onChange={(e) => setFilterPool(e.target.value)}>
          <option value="">全部资源池</option>
          {pools.map((p) => (
            <option key={p.id} value={p.id}>
              {p.name}
            </option>
          ))}
        </select>
        {auth.status === "ready" && auth.me.role === "admin" && (
          <select value={filterTenant} onChange={(e) => setFilterTenant(e.target.value)}>
            <option value="">全部租户</option>
            {tenants.map((t) => (
              <option key={t} value={t}>
                {t}
              </option>
            ))}
          </select>
        )}
        <button type="button" className="btn ghost" onClick={() => void load()}>
          刷新
        </button>
        <button type="button" className="btn primary" onClick={() => setWizardOpen(true)}>
          申请切片
        </button>
      </div>

      {err && <div className="alert error">{err}</div>}

      <div className="tabs">
        {(
          [
            ["templates", "切片模板"],
            ["topology", "拓扑视图"],
            ["quota", "配额账本"],
            ["mine", "我的分配"],
          ] as const
        ).map(([k, label]) => (
          <button
            key={k}
            type="button"
            className={tab === k ? "tab active" : "tab"}
            onClick={() => setTab(k)}
          >
            {label}
          </button>
        ))}
      </div>

      {tab === "templates" && (
        <div className="card">
          <h3>切片规格（Flavor）</h3>
          <table className="data">
            <thead>
              <tr>
                <th>名称</th>
                <th>硬件族</th>
                <th>算力核</th>
                <th>显存 MiB</th>
                <th>K8s 扩展资源</th>
                <th>虚拟化</th>
              </tr>
            </thead>
            <tbody>
              {flavors.map((f) => (
                <tr key={f.id}>
                  <td>
                    <strong>{f.display_name}</strong>
                    <div className="small muted">{f.id}</div>
                  </td>
                  <td>{f.hardware_family}</td>
                  <td>{f.ai_cores}</td>
                  <td>{f.memory_mib}</td>
                  <td>
                    <code>{f.k8s_extended_resource}</code>
                  </td>
                  <td>{f.virtualization_mode || "—"}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {tab === "topology" && (
        <div className="card">
          <h3>池 → 节点 → 切片</h3>
          <div className="tree">
            {[...nodesByPool.entries()].map(([poolId, poolNodes]) => (
              <div key={poolId}>
                <div className="pool">
                  {poolMap.get(poolId)?.name || poolId}
                  <span className="muted small"> ({poolId})</span>
                </div>
                {poolNodes.map((n) => (
                  <div key={n.name}>
                    <div className="node">
                      节点 {n.name} · {n.cards.length} 卡
                    </div>
                    {n.slices.map((s) => (
                      <div className="slice" key={s.id}>
                        {s.available ? "○" : "●"} {s.id} · {flavorMap.get(s.flavor_id)?.display_name || s.flavor_id}
                      </div>
                    ))}
                  </div>
                ))}
              </div>
            ))}
          </div>
        </div>
      )}

      {tab === "quota" && (
        <div className="grid2">
          {ledger
            .filter((r) => !filterPool || r.pool_id === filterPool)
            .map((r) => {
              const usedPct =
                r.capacity_units > 0
                  ? Math.round((r.allocated_units / r.capacity_units) * 100)
                  : 0;
              return (
                <div className="card" key={`${r.pool_id}-${r.flavor_id}`}>
                  <div className="small muted">{poolMap.get(r.pool_id)?.name || r.pool_id}</div>
                  <strong>{flavorMap.get(r.flavor_id)?.display_name || r.flavor_id}</strong>
                  <div className="row" style={{ marginTop: "0.5rem" }}>
                    <span>
                      已用 {r.allocated_units} / {r.capacity_units}
                    </span>
                    <span className="muted">空闲 {r.free_units}</span>
                  </div>
                  <div className="progress">
                    <i style={{ width: `${usedPct}%` }} />
                  </div>
                </div>
              );
            })}
        </div>
      )}

      {tab === "mine" && (
        <div className="card">
          <h3>分配记录</h3>
          <table className="data">
            <thead>
              <tr>
                <th>ID</th>
                <th>租户</th>
                <th>池 / 规格</th>
                <th>切片单元</th>
                <th>阶段</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {filteredAllocs.map((a) => (
                <tr key={a.id}>
                  <td>
                    <code>{a.id}</code>
                  </td>
                  <td>{a.tenant}</td>
                  <td>
                    {poolMap.get(a.pool_id)?.name || a.pool_id}
                    <br />
                    <span className="small muted">{a.flavor_id}</span>
                  </td>
                  <td>{a.slice_unit_id || "—"}</td>
                  <td>
                    <span className={phaseBadge(a.phase)}>{a.phase}</span>
                  </td>
                  <td>
                    {a.phase !== "Released" && (
                      <button
                        type="button"
                        className="btn ghost"
                        disabled={busy}
                        onClick={() => void release(a.id)}
                      >
                        释放
                      </button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {wizardOpen && (
        <>
          <div className="drawer-backdrop" onClick={() => setWizardOpen(false)} />
          <div className="drawer">
            <h2>申请切片</h2>
            <div className="wizard-steps">
              <span className="on">1 规格</span>
              <span className="on">2 资源池</span>
              <span className="on">3 预览 K8s 约束</span>
            </div>
            <label>
              资源池
              <select value={wizPool} onChange={(e) => setWizPool(e.target.value)}>
                {pools.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.name}
                  </option>
                ))}
              </select>
            </label>
            <label>
              切片规格
              <select value={wizFlavor} onChange={(e) => setWizFlavor(e.target.value)}>
                {supportedFlavors.map((f) => (
                  <option key={f.id} value={f.id}>
                    {f.display_name}
                  </option>
                ))}
              </select>
            </label>
            <label>
              命名空间
              <input value={wizNs} onChange={(e) => setWizNs(e.target.value)} />
            </label>
            <div className="row">
              <button type="button" className="btn" onClick={() => void previewBinding()}>
                预览 K8s 绑定
              </button>
              <button type="button" className="btn primary" disabled={busy} onClick={() => void submitAlloc()}>
                提交申请
              </button>
              <button type="button" className="btn ghost" onClick={() => setWizardOpen(false)}>
                取消
              </button>
            </div>
            {preview && (
              <pre style={{ marginTop: "1rem", overflow: "auto" }}>{preview}</pre>
            )}
            {(() => {
              const row = ledger.find((r) => r.pool_id === wizPool && r.flavor_id === wizFlavor);
              if (!row) return null;
              return (
                <div className="alert info">
                  预计排队：当前空闲 {row.free_units} 单元，已分配 {row.allocated_units} / {row.capacity_units}
                </div>
              );
            })()}
          </div>
        </>
      )}
    </>
  );
}
