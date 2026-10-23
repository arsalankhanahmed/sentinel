import { useState } from "react";
import { Link, useParams } from "react-router-dom";
import { post } from "../api/client";
import { useFetch } from "../hooks/useFetch";
import DataGrid from "../components/DataGrid";
import { SeverityBadge, StatusBadge, ago } from "../components/Badges";
import { KIND_LABEL, type Incident } from "../types";

/**
 * Deep-link target: /i/:token
 * Resolves the opaque token server-side (hash lookup) → full incident incl. captured rows.
 * Raw data is only ever rendered here, behind authentication — never inside notifications.
 */
export default function IncidentPage() {
  const { token } = useParams();
  const { data: inc, error, loading, reload } = useFetch<Incident>(`/incidents/link/${token}`, 15000);
  const [busy, setBusy] = useState(false);
  const [done, setDone] = useState<Record<number, boolean>>({});

  if (loading && !inc) return <div className="text-sm text-slate-500">Loading incident…</div>;
  if (error || !inc)
    return (
      <div className="card mx-auto mt-16 max-w-md p-8 text-center">
        <h1 className="text-lg font-semibold">Incident link not found</h1>
        <p className="mt-2 text-sm text-slate-400">The link is invalid or has expired.</p>
        <Link to="/" className="btn-ghost mt-6">Go to incidents</Link>
      </div>
    );

  const act = async (what: "ack" | "resolve") => {
    setBusy(true);
    try { await post(`/incidents/${inc.id}/${what}`); await reload(); } finally { setBusy(false); }
  };
  const steps = inc.corrective_action.split("\n").map((s) => s.trim()).filter(Boolean);

  return (
    <div className="mx-auto max-w-7xl space-y-6">
      {/* Header */}
      <header className="flex flex-wrap items-start gap-4">
        <div className="min-w-0 flex-1 basis-full sm:basis-auto">
          <div className="flex flex-wrap items-center gap-2 text-xs text-slate-400">
            <SeverityBadge s={inc.severity} />
            <StatusBadge s={inc.status} />
            <span>{KIND_LABEL[inc.kind]}</span>
            <span>·</span>
            <span className="font-mono">{inc.id.slice(0, 8)}</span>
          </div>
          <h1 className="mt-2 text-2xl font-semibold tracking-tight">{inc.title}</h1>
          <p className="mt-1 text-sm text-slate-400">{inc.summary}</p>
        </div>
        <div className="flex gap-2">
          {inc.status === "open" && <button disabled={busy} className="btn-ghost" onClick={() => act("ack")}>Acknowledge</button>}
          {inc.status !== "resolved" && <button disabled={busy} className="btn-primary" onClick={() => act("resolve")}>Resolve</button>}
        </div>
      </header>

      {/* Facts */}
      <dl className="grid grid-cols-2 gap-3 md:grid-cols-5">
        {[
          ["Source", inc.source_name],
          ["Opened", `${new Date(inc.opened_at).toLocaleString()} (${ago(inc.opened_at)})`],
          ["Last seen", ago(inc.last_seen_at)],
          ["Rows captured", `${inc.row_count}${inc.truncated ? " (truncated)" : ""}`],
          ["Occurrences", inc.occurrences],
        ].map(([k, v]) => (
          <div key={k as string} className="card p-3">
            <dt className="label">{k}</dt>
            <dd className="truncate text-sm text-slate-100">{v}</dd>
          </div>
        ))}
      </dl>

      <div className="grid gap-6 lg:grid-cols-[1fr_22rem]">
        {/* Exact captured exception rows */}
        <section className="min-w-0">
          <h2 className="mb-2 text-sm font-semibold text-slate-300">Captured exception rows</h2>
          <DataGrid columns={inc.columns ?? []} rows={inc.payload ?? []} />
        </section>

        {/* Recommended Corrective Action Plan */}
        <aside>
          <h2 className="mb-2 text-sm font-semibold text-slate-300">Recommended corrective action plan</h2>
          <ol className="card divide-y divide-slate-800">
            {steps.map((s, i) => (
              <li key={i} className="flex gap-3 p-3">
                <input
                  type="checkbox"
                  className="mt-0.5 h-4 w-4 accent-sky-500"
                  checked={!!done[i]}
                  onChange={(e) => setDone((d) => ({ ...d, [i]: e.target.checked }))}
                />
                <span className={`text-sm ${done[i] ? "text-slate-500 line-through" : "text-slate-200"}`}>
                  <span className="mr-1 text-slate-500">{i + 1}.</span>{s}
                </span>
              </li>
            ))}
          </ol>
          {inc.acknowledged_by && <p className="mt-3 text-xs text-slate-500">Acknowledged by {inc.acknowledged_by}</p>}
          <p className="mt-1 text-xs text-slate-600">Link expires {new Date(inc.link_expires_at).toLocaleDateString()}</p>
        </aside>
      </div>
    </div>
  );
}
