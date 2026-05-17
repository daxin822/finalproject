import { useState } from "react";
import { Navigate } from "react-router-dom";
import { useAuth } from "../auth/AuthContext";

export function LoginPage() {
  const { auth, login } = useAuth();
  const [u, setU] = useState("admin");
  const [p, setP] = useState("admin");
  const [err, setErr] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  if (auth.status === "loading") {
    return (
      <div className="login-wrap">
        <p className="muted">校验会话…</p>
      </div>
    );
  }
  if (auth.status === "ready") {
    return <Navigate to="/resources" replace />;
  }

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setErr(null);
    setBusy(true);
    try {
      await login(u, p);
    } catch (ex) {
      setErr(ex instanceof Error ? ex.message : String(ex));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="login-wrap">
      <form className="card login-card" onSubmit={onSubmit}>
        <h1>登录</h1>
        <p className="muted small">
          演示账号：<code>admin</code> / <code>tenant1</code>（密码同用户名）。开发环境若关闭鉴权将以内置
          admin 处理请求。
        </p>
        <label>
          用户名
          <input value={u} onChange={(e) => setU(e.target.value)} autoComplete="username" />
        </label>
        <label>
          密码
          <input
            type="password"
            value={p}
            onChange={(e) => setP(e.target.value)}
            autoComplete="current-password"
          />
        </label>
        {err && <div className="alert error">{err}</div>}
        <button type="submit" className="btn primary" disabled={busy}>
          {busy ? "登录中…" : "进入控制台"}
        </button>
      </form>
    </div>
  );
}
