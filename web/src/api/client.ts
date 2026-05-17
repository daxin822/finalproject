const TOKEN_KEY = "cp_access_token";

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY);
}

export function setToken(t: string | null) {
  if (t) localStorage.setItem(TOKEN_KEY, t);
  else localStorage.removeItem(TOKEN_KEY);
}

export async function apiFetch<T>(
  path: string,
  init: RequestInit & { json?: unknown } = {},
): Promise<T> {
  const headers: Record<string, string> = {
    Accept: "application/json",
    ...(init.headers as Record<string, string> | undefined),
  };
  const tok = getToken();
  if (tok) headers.Authorization = `Bearer ${tok}`;
  let body: BodyInit | undefined = init.body ?? undefined;
  if (init.json !== undefined) {
    headers["Content-Type"] = "application/json";
    body = JSON.stringify(init.json);
  }
  const res = await fetch(path, { ...init, headers, body });
  const ct = res.headers.get("content-type") || "";
  if (!res.ok) {
    let detail = res.statusText;
    try {
      if (ct.includes("json")) {
        const j = (await res.json()) as { error?: string };
        if (j?.error) detail = j.error;
      } else {
        detail = await res.text();
      }
    } catch {
      /* ignore */
    }
    throw new Error(`${res.status}: ${detail}`);
  }
  if (res.status === 204) return undefined as T;
  if (ct.includes("json")) return (await res.json()) as T;
  return (await res.text()) as T;
}

export function newIdempotencyKey(): string {
  if (typeof crypto !== "undefined" && crypto.randomUUID) {
    return crypto.randomUUID();
  }
  return `idem-${Date.now()}-${Math.random().toString(16).slice(2)}`;
}
