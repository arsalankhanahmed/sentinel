// Thin, dependency-free API client. Every call carries the bearer token
// (zero-trust: deep links alone never grant data access).

const BASE = import.meta.env.VITE_API_BASE ?? "/api/v1";
const TOKEN_KEY = "sentinel.token";

export const auth = {
  get: () => {
    try { return localStorage.getItem(TOKEN_KEY); } catch { return null; }
  },
  set: (t: string) => {
    try { localStorage.setItem(TOKEN_KEY, t); } catch { /* ignore */ }
  },
  clear: () => {
    try { localStorage.removeItem(TOKEN_KEY); } catch { /* ignore */ }
  },
};

export class ApiError extends Error {
  constructor(public status: number, message: string) { super(message); }
}

export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  if (init.body) headers.set("Content-Type", "application/json");
  const t = auth.get();
  if (t) headers.set("Authorization", `Bearer ${t}`);

  const res = await fetch(BASE + path, { ...init, headers, referrerPolicy: "no-referrer" });
  if (res.status === 204) return undefined as T;
  const body = await res.json().catch(() => ({}));
  if (!res.ok) throw new ApiError(res.status, body.error ?? res.statusText);
  return body as T;
}

export const get = <T,>(p: string) => api<T>(p);
export const post = <T,>(p: string, b?: unknown) => api<T>(p, { method: "POST", body: b ? JSON.stringify(b) : undefined });
export const put = <T,>(p: string, b: unknown) => api<T>(p, { method: "PUT", body: JSON.stringify(b) });
export const del = (p: string) => api<void>(p, { method: "DELETE" });
