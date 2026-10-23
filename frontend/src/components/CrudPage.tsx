import { useState, type FormEvent, type ReactNode } from "react";
import { del, post } from "../api/client";
import { useFetch } from "../hooks/useFetch";

export type Field = {
  key: string;
  label: string;
  type?: "text" | "number" | "select" | "textarea" | "password" | "json" | "checkbox";
  options?: { value: string; label: string }[];
  placeholder?: string;
  default?: string | number | boolean;
  required?: boolean;
  wide?: boolean;
  hint?: string;
};

type Item = { id: number } & Record<string, unknown>;

/** Generic config page: list + create form + delete + optional per-row action. */
export default function CrudPage({
  title, intro, endpoint, fields, columns, rowAction, transform,
}: {
  title: string;
  intro?: ReactNode;
  endpoint: string;
  fields: Field[];
  columns: { key: string; label: string; render?: (it: Item) => ReactNode }[];
  rowAction?: { label: string; run: (it: Item) => Promise<string> };
  transform?: (v: Record<string, unknown>) => Record<string, unknown>;
}) {
  const list = useFetch<Item[]>(endpoint);
  const init = () => Object.fromEntries(fields.map((f) => [f.key, f.default ?? (f.type === "checkbox" ? false : "")]));
  const [form, setForm] = useState<Record<string, unknown>>(init);
  const [err, setErr] = useState<string | null>(null);
  const [note, setNote] = useState<Record<number, string>>({});
  const [saving, setSaving] = useState(false);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setErr(null);
    setSaving(true);
    try {
      const body: Record<string, unknown> = {};
      for (const f of fields) {
        const v = form[f.key];
        body[f.key] = f.type === "number" ? Number(v) : f.type === "json" ? JSON.parse(String(v || "{}")) : v;
      }
      await post(endpoint, transform ? transform(body) : body);
      setForm(init());
      list.reload();
    } catch (e) {
      setErr((e as Error).message);
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="mx-auto max-w-6xl space-y-6">
      <div>
        <h1 className="text-2xl font-semibold tracking-tight">{title}</h1>
        {intro && <div className="mt-1 text-sm text-slate-400">{intro}</div>}
      </div>

      <form onSubmit={submit} className="card grid gap-4 p-5 md:grid-cols-2">
        {fields.map((f) => (
          <div key={f.key} className={f.wide || f.type === "textarea" || f.type === "json" ? "md:col-span-2" : ""}>
            <label className="label">{f.label}</label>
            {f.type === "select" ? (
              <select className="input" value={String(form[f.key])} required={f.required}
                onChange={(e) => setForm({ ...form, [f.key]: e.target.value })}>
                <option value="" disabled>Select…</option>
                {f.options?.map((o) => <option key={o.value} value={o.value}>{o.label}</option>)}
              </select>
            ) : f.type === "textarea" || f.type === "json" ? (
              <textarea className="input min-h-28 font-mono" placeholder={f.placeholder} required={f.required}
                value={String(form[f.key])} onChange={(e) => setForm({ ...form, [f.key]: e.target.value })} />
            ) : f.type === "checkbox" ? (
              <input type="checkbox" className="h-4 w-4 accent-sky-500" checked={!!form[f.key]}
                onChange={(e) => setForm({ ...form, [f.key]: e.target.checked })} />
            ) : (
              <input className="input" type={f.type ?? "text"} placeholder={f.placeholder} required={f.required}
                value={String(form[f.key])} onChange={(e) => setForm({ ...form, [f.key]: e.target.value })} />
            )}
            {f.hint && <p className="mt-1 text-xs text-slate-500">{f.hint}</p>}
          </div>
        ))}
        <div className="flex items-center gap-3 md:col-span-2">
          <button className="btn-primary" disabled={saving}>{saving ? "Saving…" : "Add"}</button>
          {err && <span className="text-sm text-rose-400">{err}</span>}
        </div>
      </form>

      <div className="card overflow-x-auto">
        <table className="min-w-full text-sm">
          <thead className="text-left text-xs uppercase tracking-wide text-slate-500">
            <tr>{columns.map((c) => <th key={c.key} className="px-4 py-3">{c.label}</th>)}<th /></tr>
          </thead>
          <tbody className="divide-y divide-slate-800">
            {list.data?.map((it) => (
              <tr key={it.id}>
                {columns.map((c) => (
                  <td key={c.key} className="max-w-xs truncate px-4 py-3 text-slate-200">{c.render ? c.render(it) : String(it[c.key] ?? "")}</td>
                ))}
                <td className="whitespace-nowrap px-4 py-3 text-right">
                  {note[it.id] && <span className="mr-2 text-xs text-slate-400">{note[it.id]}</span>}
                  {rowAction && (
                    <button className="btn-ghost mr-1" onClick={async () => {
                      setNote((n) => ({ ...n, [it.id]: "…" }));
                      const r = await rowAction.run(it).catch((e: Error) => e.message);
                      setNote((n) => ({ ...n, [it.id]: r }));
                    }}>{rowAction.label}</button>
                  )}
                  <button className="btn-danger" onClick={async () => {
                    if (confirm("Delete this item?")) { await del(`${endpoint}/${it.id}`); list.reload(); }
                  }}>Delete</button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {list.data?.length === 0 && <div className="p-6 text-center text-sm text-slate-500">Nothing configured yet.</div>}
      </div>
    </div>
  );
}
