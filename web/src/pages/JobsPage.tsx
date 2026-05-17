import { useCallback, useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import * as api from "../api/endpoints";
import { useAuth } from "../auth/AuthContext";
import type { SliceFlavor, WorkloadRecord } from "../types";

type JobTab = "list" | "train" | "infer";

function phaseClass(phase: string) {
  if (phase === "Running" || phase === "Succeeded") return "badge ok";
  if (phase === "Failed") return "badge warn";
  return "badge";
}

export function JobsPage() {
  const { auth } = useAuth();
  const defaultTenant =
    auth.status === "ready" && auth.me.role === "tenant" ? auth.me.tenant : "tenant1";

  const [tab, setTab] = useState<JobTab>("list");
  const [workloads, setWorkloads] = useState<WorkloadRecord[]>([]);
  const [flavors, setFlavors] = useState<SliceFlavor[]>([]);
  const [err, setErr] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [detail, setDetail] = useState<WorkloadRecord | null>(null);

  const [name, setName] = useState("demo-train");
  const [ns, setNs] = useState("default");
  const [image, setImage] = useState("busybox:latest");
  const [flavorId, setFlavorId] = useState("");
  const [extRes, setExtRes] = useState("");
  const [replicas, setReplicas] = useState(1);
  const [command, setCommand] = useState("sleep 3600");
  const [dist, setDist] = useState(false);
  const [worldSize, setWorldSize] = useState(2);
  const [yamlPreview, setYamlPreview] = useState("");
  const [inferPort, setInferPort] = useState(8080);
  const [createSvc, setCreateSvc] = useState(true);

  const load = useCallback(async () => {
    setErr(null);
    try {
      const [w, f] = await Promise.all([api.listWorkloads(), api.listFlavors()]);
      setWorkloads(w.workloads || []);
      setFlavors(f);
      if (!flavorId && f[0]) {
        setFlavorId(f[0].id);
        setExtRes(f[0].k8s_extended_resource);
      }
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
    }
  }, [flavorId]);

  useEffect(() => {
    void load();
  }, [load]);

  const flavorMap = useMemo(() => {
    const m = new Map<string, SliceFlavor>();
    flavors.forEach((x) => m.set(x.id, x));
    return m;
  }, [flavors]);

  function onFlavorChange(fid: string) {
    setFlavorId(fid);
    const f = flavorMap.get(fid);
    if (f) setExtRes(f.k8s_extended_resource);
  }

  async function openDetail(id: string) {
    try {
      const w = await api.getWorkload(id);
      setDetail(w);
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
    }
  }

  async function removeJob(id: string) {
    if (!confirm("删除任务及集群对象？")) return;
    setBusy(true);
    try {
      await api.deleteWorkload(id);
      if (detail?.id === id) setDetail(null);
      await load();
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  }

  function buildTrainSpec() {
    const cmd = command.trim() ? command.trim().split(/\s+/) : undefined;
    return {
      tenant: defaultTenant,
      name,
      namespace: ns,
      image,
      flavor_id: flavorId || undefined,
      extended_resource: extRes,
      replicas: dist ? worldSize : replicas,
      min_available: dist ? worldSize : 1,
      command: cmd,
      distributed: dist
        ? { enabled: true, world_size: worldSize }
        : undefined,
    };
  }

  function buildInferSpec() {
    const cmd = command.trim() ? command.trim().split(/\s+/) : undefined;
    return {
      tenant: defaultTenant,
      name: name.replace(/^train/, "infer") || "demo-infer",
      namespace: ns,
      image,
      flavor_id: flavorId || undefined,
      extended_resource: extRes,
      replicas,
      container_port: inferPort,
      create_service: createSvc,
      service_type: "ClusterIP",
      command: cmd,
    };
  }

  async function previewYaml() {
    setBusy(true);
    try {
      if (tab === "infer") {
        const r = await api.renderInference(buildInferSpec());
        setYamlPreview(r.yaml);
      } else {
        const r = await api.renderTraining(buildTrainSpec());
        setYamlPreview(r.yaml);
      }
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  }

  async function submitJob() {
    setBusy(true);
    setErr(null);
    try {
      if (tab === "infer") {
        await api.submitInference(buildInferSpec());
      } else {
        await api.submitTraining(buildTrainSpec());
      }
      setTab("list");
      await load();
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  }

  function logUrl(w: WorkloadRecord) {
    const pod = w.pod_names?.[0] || w.name;
    return `# 日志入口（示例 kubectl）\nkubectl logs -n ${w.namespace} ${pod} -f`;
  }

  return (
    <>
      <h1 className="page-title">训推任务</h1>
      <p className="page-desc">
        表单化提交训练（Volcano Job）与推理（Deployment），查看状态时间线与 Events 摘要，并跳转日志命令。
      </p>

      {err && <div className="alert error">{err}</div>}

      <div className="tabs">
        <button
          type="button"
          className={tab === "list" ? "tab active" : "tab"}
          onClick={() => setTab("list")}
        >
          任务列表
        </button>
        <button
          type="button"
          className={tab === "train" ? "tab active" : "tab"}
          onClick={() => setTab("train")}
        >
          提交训练
        </button>
        <button
          type="button"
          className={tab === "infer" ? "tab active" : "tab"}
          onClick={() => setTab("infer")}
        >
          提交推理
        </button>
      </div>

      {tab === "list" && (
        <div className="card">
          <div className="row" style={{ marginBottom: "0.75rem" }}>
            <button type="button" className="btn ghost" onClick={() => void load()}>
              刷新
            </button>
            <Link to="/cosched" className="btn ghost">
              查看协同冲突
            </Link>
          </div>
          <table className="data">
            <thead>
              <tr>
                <th>名称</th>
                <th>类型</th>
                <th>租户</th>
                <th>阶段</th>
                <th>规格</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {workloads.map((w) => (
                <tr key={w.id}>
                  <td>
                    <strong>{w.name}</strong>
                    <div className="small muted">{w.id}</div>
                  </td>
                  <td>{w.kind}</td>
                  <td>{w.tenant || "—"}</td>
                  <td>
                    <span className={phaseClass(w.phase)}>{w.phase}</span>
                  </td>
                  <td className="small">{w.flavor_id || w.extended_resource || "—"}</td>
                  <td>
                    <button type="button" className="btn ghost" onClick={() => void openDetail(w.id)}>
                      详情
                    </button>
                    <button
                      type="button"
                      className="btn ghost"
                      disabled={busy}
                      onClick={() => void removeJob(w.id)}
                    >
                      删除
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {(tab === "train" || tab === "infer") && (
        <div className="grid2">
          <div className="card">
            <h3>{tab === "train" ? "训练向导" : "推理向导"}</h3>
            <div className="wizard-steps">
              <span className="on">基础</span>
              <span className="on">资源</span>
              <span className="on">预览 YAML</span>
            </div>
            <label>
              任务名
              <input value={name} onChange={(e) => setName(e.target.value)} />
            </label>
            <label>
              命名空间
              <input value={ns} onChange={(e) => setNs(e.target.value)} />
            </label>
            <label>
              镜像
              <input value={image} onChange={(e) => setImage(e.target.value)} />
            </label>
            <label>
              切片规格
              <select value={flavorId} onChange={(e) => onFlavorChange(e.target.value)}>
                {flavors.map((f) => (
                  <option key={f.id} value={f.id}>
                    {f.display_name}
                  </option>
                ))}
              </select>
            </label>
            <label>
              扩展资源
              <input value={extRes} onChange={(e) => setExtRes(e.target.value)} />
            </label>
            {tab === "train" && (
              <>
                <label>
                  副本 / 规模
                  <input
                    type="number"
                    min={1}
                    value={dist ? worldSize : replicas}
                    onChange={(e) => {
                      const v = Number(e.target.value);
                      if (dist) setWorldSize(v);
                      else setReplicas(v);
                    }}
                  />
                </label>
                <label className="row">
                  <input type="checkbox" checked={dist} onChange={(e) => setDist(e.target.checked)} />
                  分布式训练
                </label>
              </>
            )}
            {tab === "infer" && (
              <>
                <label>
                  副本数
                  <input
                    type="number"
                    min={1}
                    value={replicas}
                    onChange={(e) => setReplicas(Number(e.target.value))}
                  />
                </label>
                <label>
                  容器端口
                  <input
                    type="number"
                    value={inferPort}
                    onChange={(e) => setInferPort(Number(e.target.value))}
                  />
                </label>
                <label className="row">
                  <input
                    type="checkbox"
                    checked={createSvc}
                    onChange={(e) => setCreateSvc(e.target.checked)}
                  />
                  创建 Service
                </label>
              </>
            )}
            <label>
              启动命令（空格分隔）
              <input value={command} onChange={(e) => setCommand(e.target.value)} />
            </label>
            <div className="row">
              <button type="button" className="btn" disabled={busy} onClick={() => void previewYaml()}>
                预览 YAML
              </button>
              <button type="button" className="btn primary" disabled={busy} onClick={() => void submitJob()}>
                提交
              </button>
            </div>
          </div>
          <div className="card">
            <h3>渲染结果</h3>
            {yamlPreview ? (
              <pre style={{ overflow: "auto", maxHeight: "480px" }}>{yamlPreview}</pre>
            ) : (
              <p className="muted">点击「预览 YAML」查看 MindCluster 模板输出。</p>
            )}
          </div>
        </div>
      )}

      {detail && (
        <>
          <div className="drawer-backdrop" onClick={() => setDetail(null)} />
          <div className="drawer">
            <h2>{detail.name}</h2>
            <p className="muted small">
              {detail.kind} · {detail.namespace} · {detail.id}
            </p>
            <p>
              阶段：<span className={phaseClass(detail.phase)}>{detail.phase}</span>
            </p>
            {detail.message && <div className="alert info">{detail.message}</div>}
            <h3>状态时间线</h3>
            <ul className="small">
              <li>创建 {new Date(detail.created_at).toLocaleString()}</li>
              <li>更新 {new Date(detail.updated_at).toLocaleString()}</li>
              {detail.pod_names?.map((p) => (
                <li key={p}>Pod {p}</li>
              ))}
            </ul>
            {detail.events_summary && detail.events_summary.length > 0 && (
              <>
                <h3>Events 摘要</h3>
                <ul className="small">
                  {detail.events_summary.map((ev, i) => (
                    <li key={i}>{ev}</li>
                  ))}
                </ul>
              </>
            )}
            <h3>日志入口</h3>
            <pre>{logUrl(detail)}</pre>
            <div className="row">
              <button type="button" className="btn ghost" onClick={() => setDetail(null)}>
                关闭
              </button>
              <button
                type="button"
                className="btn"
                disabled={busy}
                onClick={() => void removeJob(detail.id)}
              >
                停止并删除
              </button>
            </div>
          </div>
        </>
      )}
    </>
  );
}
