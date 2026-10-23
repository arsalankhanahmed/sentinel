import { useState } from "react";
import { post } from "../api/client";
import { useFetch } from "../hooks/useFetch";
import { SeverityBadge, StatusBadge, ago } from "../components/Badges";
import { KIND_LABEL, type Incident, type IncidentKind, type Stats } from "../types";

const KINDS: IncidentKind[] = ["business_exception", "runtime_exception", "db_outage", "endpoint_outage"];

export default function Dashboard() {
  const [status, setStatus] = useState<"" | "open" | "acknowledged" | "resolved">("open");
  const stats = useFetch<Stats>("/stats", 10000);
  const list = useFetch<Incident[]>(`/incidents?limit=200${status ? `&status=${status}` : ""}`, 10000);

  const setInc = async (id: string, what: "ack" | "resolve") => {
    await post(`/incidents/${id}/${what}`);
    list.reload(); stats.reload();
  };

  return (
    <div className="mx-auto max-w-7xl space-y-6">
      <h1 className="text-2xl font-semibold tracking-tight">Incidents</h1>

      <div className="grid grid-cols-2 gap-3 md:grid-cols-5">
        {KINDS.map((k) => (
          <div key={k} className="card p-4">
            <div className="label">{KIND_LABEL[k]}</div>
            <div className={`text-3xl font-semibold tabular-nums ${stats.data?.open_by_kind[k] ? "text-rose-300" : "text-slate-200"}`}>
              {stats.data?.open_by_kind[k] ?? 0}
            </div>
          </div>
        ))}
        <div className="card p-4">
          <div className="label">Last 24h</div>
          <div className="text-3xl font-semibold tabular-nums">{stats.data?.last_24h ?? 0}</div>
        </div>
      </div>

      <div className="flex gap-1">
        {(["open", "acknowledged", "resolved", ""] as const).map((s) => (
          <button key={s || "all"} onClick={() => setStatus(s)}
            className={`btn ${status === s ? "bg-slate-800 text-white" : "text-slate-400 hover:text-slate-200"}`}>
            {s || "all"}
          </button>
        ))}
      </div>

      <div className="card overflow-x-auto">
        <table className="min-w-full text-sm">
          <thead className="text-left text-xs uppercase tracking-wide text-slate-500">
            <tr>
              <th className="px-4 py-3">Severity</th><th className="px-4 py-3">Incident</th><th className="px-4 py-3">Type</th>
              <th className="px-4 py-3">Source</th><th className="px-4 py-3">Rows</th><th className="px-4 py-3">Opened</th>
              <th className="px-4 py-3">Status</th><th className="px-4 py-3"></th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-800">
            {list.data?.map((i) => (
              <tr key={i.id} className="hover:bg-slate-800/30">
                <td className="px-4 py-3"><SeverityBadge s={i.severity} /></td>
                <td className="max-w-xs px-4 py-3">
                  <div className="truncate font-medium text-slate-100">{i.title}</div>
                  {i.occurrences > 1 && <div className="text-xs text-slate-500">seen {i.occurrences}× · last {ago(i.last_seen_at)}</div>}
                </td>
                <td className="whitespace-nowrap px-4 py-3 text-slate-400">{KIND_LABEL[i.kind]}</td>
                <td className="px-4 py-3 text-slate-300">{i.source_name}</td>
                <td className="px-4 py-3 tabular-nums">{i.row_count}</td>
                <td className="whitespace-nowrap px-4 py-3 text-slate-400">{ago(i.opened_at)}</td>
                <td className="px-4 py-3"><StatusBadge s={i.status} /></td>
                <td className="whitespace-nowrap px-4 py-3 text-right">
                  {i.status === "open" && <button className="btn-ghost mr-1" onClick={() => setInc(i.id, "ack")}>Ack</button>}
                  {i.status !== "resolved" && <button className="btn-ghost" onClick={() => setInc(i.id, "resolve")}>Resolve</button>}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {list.data?.length === 0 && <div className="p-8 text-center text-sm text-slate-500">All clear — no incidents.</div>}
        {list.error && <div className="p-4 text-sm text-rose-400">{list.error}</div>}
      </div>
      <p className="text-xs text-slate-600">Captured rows open from the deep link in each notification (Teams / Zoom / WhatsApp / Email).</p>
    </div>
  );
}
