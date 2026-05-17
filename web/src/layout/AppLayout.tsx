import { NavLink, Outlet, Navigate } from "react-router-dom";
import { useAuth } from "../auth/AuthContext";

const nav = [
  { to: "/resources", label: "切片与资源池" },
  { to: "/jobs", label: "训推任务" },
  { to: "/cosched", label: "训推协同" },
  { to: "/dashboard", label: "监控大盘" },
];

export function AppLayout() {
  const { auth, logout } = useAuth();
  if (auth.status === "loading") {
    return (
      <div className="shell">
        <p className="muted">加载中…</p>
      </div>
    );
  }
  if (auth.status === "anonymous") {
    return <Navigate to="/login" replace />;
  }

  return (
    <div className="app-root">
      <aside className="sidebar">
        <div className="brand">
          <strong>昇腾资源池</strong>
          <span className="muted small">控制面控制台</span>
        </div>
        <nav className="nav">
          {nav.map((n) => (
            <NavLink
              key={n.to}
              to={n.to}
              className={({ isActive }) => (isActive ? "navlink active" : "navlink")}
            >
              {n.label}
            </NavLink>
          ))}
        </nav>
        <div className="sidebar-footer">
          <div className="small muted">
            {auth.me.sub} · {auth.me.role}
            {auth.me.tenant ? ` · ${auth.me.tenant}` : ""}
          </div>
          <button type="button" className="btn ghost" onClick={logout}>
            退出
          </button>
        </div>
      </aside>
      <main className="main">
        <Outlet />
      </main>
    </div>
  );
}
