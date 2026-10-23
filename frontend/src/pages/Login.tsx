import { useState, type FormEvent } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { auth, get } from "../api/client";

/**
 * Minimal sign-in: paste a bearer token issued by your IdP / token service.
 * In production replace with an OIDC redirect (Entra ID, Okta, Keycloak) — the
 * `from` state preserves the deep link so the engineer lands on the incident.
 */
export default function Login() {
  const [token, setToken] = useState("");
  const [err, setErr] = useState<string | null>(null);
  const nav = useNavigate();
  const from = (useLocation().state as { from?: string } | null)?.from ?? "/";

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    auth.set(token.trim());
    try {
      await get("/stats");
      nav(from, { replace: true });
    } catch {
      auth.clear();
      setErr("Token rejected.");
    }
  };

  return (
    <div className="grid min-h-full place-items-center p-4">
      <form onSubmit={submit} className="card w-full max-w-sm space-y-4 p-6">
        <div className="flex items-center gap-2">
          <span className="grid h-8 w-8 place-items-center rounded-md bg-sky-600 text-sm font-bold text-white">S</span>
          <h1 className="text-lg font-semibold">Sentinel sign-in</h1>
        </div>
        <div>
          <label className="label">Access token</label>
          <textarea className="input min-h-24 font-mono text-xs" value={token} onChange={(e) => setToken(e.target.value)} required />
        </div>
        {from !== "/" && <p className="text-xs text-slate-400">You'll be taken to the incident after signing in.</p>}
        {err && <p className="text-sm text-rose-400">{err}</p>}
        <button className="btn-primary w-full justify-center">Sign in</button>
      </form>
    </div>
  );
}
