import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { setToken } from "../api/client";
import * as api from "../api/endpoints";
import type { MeResponse } from "../types";

type AuthState =
  | { status: "loading" }
  | { status: "anonymous" }
  | { status: "ready"; me: MeResponse };

const AuthCtx = createContext<{
  auth: AuthState;
  refresh: () => Promise<void>;
  login: (u: string, p: string) => Promise<void>;
  logout: () => void;
} | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [auth, setAuth] = useState<AuthState>({ status: "loading" });

  const refresh = useCallback(async () => {
    try {
      const me = await api.me();
      setAuth({ status: "ready", me });
    } catch {
      setToken(null);
      setAuth({ status: "anonymous" });
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const login = useCallback(async (username: string, password: string) => {
    const r = await api.login(username, password);
    if (r.access_token) setToken(r.access_token);
    else setToken(null);
    await refresh();
  }, [refresh]);

  const logout = useCallback(() => {
    setToken(null);
    setAuth({ status: "anonymous" });
  }, []);

  const v = useMemo(
    () => ({ auth, refresh, login, logout }),
    [auth, refresh, login, logout],
  );

  return <AuthCtx.Provider value={v}>{children}</AuthCtx.Provider>;
}

export function useAuth() {
  const x = useContext(AuthCtx);
  if (!x) throw new Error("useAuth outside provider");
  return x;
}
