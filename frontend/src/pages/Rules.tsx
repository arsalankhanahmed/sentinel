import { useState, type FormEvent } from "react";
import { del, post, put } from "../api/client";
import { useFetch } from "../hooks/useFetch";
import ScheduleBuilder from "../components/ScheduleBuilder";
import { SeverityBadge, ago } from "../components/Badges";
import type { DataSource, Rule, Severity } from "../types";

const empty = {
  name: "", data_source_id: 0, sql_text: "", schedule: "*/5 * * * *",
  severity: "warning" as Severity, corrective_action: "", max_rows: 500, enabled: true,
};

export default function Rules() {
  const rules = useFetch<Rule[]>("/rules", 15000);
  const ds = useFetch<DataSource[]>("/datasources");
  const [form, setForm] = useState(empty);
  const [editing, setEditing] = useState<number | null>(null);
  const [err, setErr] = useState<string | null>(null);
  const [formKey, setFormKey] = useState(0);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setErr(null);
    try {
      const body = { ...form, data_source_id: Number(form.data_source_id), max_rows: Number(form.max_rows) };
      editing ? await put(`/rules/${editing}`, body) : await post("/rules", body);
      setForm(empty); setEditing(null); setFormKey((k) => k + 1);
      rules.reload();
    } catch (e) { setErr((e as Error).message); }
  };

  const edit = (r: Rule) => {
    setEditing(r.id);
    setForm({ name: r.name, data_source_id: r.data_source_id, sql_text: r.sql_text, schedule: r.schedule,
      severity: r.severity, corrective_action: r.corrective_action, max_rows: r.max_rows, enabled: r.enabled });
    setFormKey((k) => k + 1);
    window.scrollTo({ top: 0, behavior: "smooth" });
  };

  const dsName = (id: number) => ds.data?.find((d) => d.id === id)?.name ?? `#${id}`;

  return (
    <div className="mx-auto max-w-6xl space-y-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">Business Exception Rules</h1>
        <p className="mt-1 text-sm text-slate-400">
          Write a query that returns the <em>bad</em> records. If it returns any rows, they are captured and an incident is raised.
          Runs in a read-only, always-rolled-back transaction. Auto-resolves when the query returns 0 rows.
        </p>
      </div>

      <form key={formKey} onSubmit={submit} className="card grid gap-4 p-5 md:grid-cols-2">
        <div>
          <label className="label">Rule name</label>
          <input className="input" required value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })}
            placeholder="Invoices posted without GL entry" />
        </div>
        <div>
          <label className="label">Data source</label>
          <select className="input" required value={form.data_source_id || ""}
            onChange={(e) => setForm({ ...form, data_source_id: Number(e.target.value) })}>
            <option value="" disabled>Select…</option>
            {ds.data?.map((d) => <option key={d.id} value={d.id}>{d.name} ({d.driver})</option>)}
          </select>
        </div>
        <div className="md:col-span-2">
          <label className="label">SQL script</label>
          <textarea className="input min-h-40 font-mono" required spellCheck={false} value={form.sql_text}
            onChange={(e) => setForm({ ...form, sql_text: e.target.value })}
            placeholder={"SELECT InvoiceNo, Amount, PostedAt\nFROM dbo.Invoices i\nWHERE i.Posted = 1 AND NOT EXISTS (SELECT 1 FROM dbo.GLEntries g WHERE g.InvoiceNo = i.InvoiceNo)"} />
        </div>
        <div className="md:col-span-2">
          <label className="label">Schedule</label>
          <ScheduleBuilder value={form.schedule} onChange={(schedule) => setForm((f) => ({ ...f, schedule }))} />
        </div>
        <div>
          <label className="label">Severity</label>
          <select className="input" value={form.severity} onChange={(e) => setForm({ ...form, severity: e.target.value as Severity })}>
            <option value="critical">Critical</option><option value="warning">Warning</option><option value="info">Info</option>
          </select>
        </div>
        <div>
          <label className="label">Max rows captured</label>
          <input type="number" min={1} max={10000} className="input" value={form.max_rows}
            onChange={(e) => setForm({ ...form, max_rows: Number(e.target.value) })} />
        </div>
        <div className="md:col-span-2">
          <label className="label">Recommended corrective action plan (one step per line)</label>
          <textarea className="input min-h-24" value={form.corrective_action}
            onChange={(e) => setForm({ ...form, corrective_action: e.target.value })}
            placeholder="Leave empty to use the built-in dynamic playbook" />
        </div>
        <div className="flex items-center gap-3 md:col-span-2">
          <label className="flex items-center gap-2 text-sm text-slate-300">
            <input type="checkbox" className="h-4 w-4 accent-sky-500" checked={form.enabled}
              onChange={(e) => setForm({ ...form, enabled: e.target.checked })} /> Enabled
          </label>
          <button className="btn-primary">{editing ? "Save changes" : "Create rule"}</button>
          {editing && <button type="button" className="btn-ghost" onClick={() => { setEditing(null); setForm(empty); setFormKey((k) => k + 1); }}>Cancel</button>}
          {err && <span className="text-sm text-rose-400">{err}</span>}
        </div>
      </form>

      <div className="card overflow-x-auto">
        <table className="min-w-full text-sm">
          <thead className="text-left text-xs uppercase tracking-wide text-slate-500">
            <tr><th className="px-4 py-3">Rule</th><th className="px-4 py-3">Source</th><th className="px-4 py-3">Schedule</th>
              <th className="px-4 py-3">Last run</th><th className="px-4 py-3">Result</th><th /></tr>
          </thead>
          <tbody className="divide-y divide-slate-800">
            {rules.data?.map((r) => (
              <tr key={r.id} className={r.enabled ? "" : "opacity-50"}>
                <td className="px-4 py-3"><div className="flex items-center gap-2"><SeverityBadge s={r.severity} /><span className="font-medium">{r.name}</span></div></td>
                <td className="px-4 py-3 text-slate-300">{dsName(r.data_source_id)}</td>
                <td className="px-4 py-3 font-mono text-xs text-slate-400">{r.schedule}</td>
                <td className="whitespace-nowrap px-4 py-3 text-slate-400">
                  {r.last_run_at ? `${ago(r.last_run_at)} · ${r.last_duration_ms} ms` : "never"}
                </td>
                <td className={`max-w-xs truncate px-4 py-3 text-xs ${r.last_status?.startsWith("error") ? "text-rose-400" : r.last_status?.startsWith("exception") ? "text-amber-300" : "text-emerald-400"}`}
                  title={r.last_status ?? ""}>{r.last_status ?? "—"}</td>
                <td className="whitespace-nowrap px-4 py-3 text-right">
                  <button className="btn-ghost mr-1" onClick={async () => { await post(`/rules/${r.id}/run`).catch(() => {}); setTimeout(rules.reload, 1500); }}>Run now</button>
                  <button className="btn-ghost mr-1" onClick={() => edit(r)}>Edit</button>
                  <button className="btn-danger" onClick={async () => { if (confirm("Delete rule?")) { await del(`/rules/${r.id}`); rules.reload(); } }}>Delete</button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {rules.data?.length === 0 && <div className="p-6 text-center text-sm text-slate-500">No rules yet.</div>}
      </div>
    </div>
  );
}
