import { useMemo, useState } from "react";

type Row = Record<string, unknown>;

const fmt = (v: unknown) => (v === null || v === undefined ? "" : typeof v === "object" ? JSON.stringify(v) : String(v));

/**
 * Fast, dependency-free grid for captured exception rows:
 * sticky header, column sort, full-text filter, CSV export, windowed rendering.
 */
export default function DataGrid({ columns, rows }: { columns: string[]; rows: Row[] }) {
  const [q, setQ] = useState("");
  const [sort, setSort] = useState<{ col: string; dir: 1 | -1 } | null>(null);
  const [limit, setLimit] = useState(200);

  const view = useMemo(() => {
    const needle = q.trim().toLowerCase();
    let r = needle ? rows.filter((row) => columns.some((c) => fmt(row[c]).toLowerCase().includes(needle))) : rows;
    if (sort) {
      r = [...r].sort((a, b) => {
        const x = a[sort.col], y = b[sort.col];
        if (typeof x === "number" && typeof y === "number") return (x - y) * sort.dir;
        return fmt(x).localeCompare(fmt(y), undefined, { numeric: true }) * sort.dir;
      });
    }
    return r;
  }, [rows, columns, q, sort]);

  const exportCsv = () => {
    const esc = (s: string) => (/[",\n]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s);
    const csv = [columns.join(","), ...view.map((r) => columns.map((c) => esc(fmt(r[c]))).join(","))].join("\n");
    const a = document.createElement("a");
    a.href = URL.createObjectURL(new Blob([csv], { type: "text/csv" }));
    a.download = "incident-rows.csv";
    a.click();
    URL.revokeObjectURL(a.href);
  };

  return (
    <div className="card overflow-hidden">
      <div className="flex flex-wrap items-center gap-2 border-b border-slate-800 p-3">
        <input className="input max-w-xs" placeholder="Filter rows…" value={q} onChange={(e) => setQ(e.target.value)} />
        <span className="text-xs text-slate-500">{view.length.toLocaleString()} of {rows.length.toLocaleString()} rows</span>
        <button className="btn-ghost ml-auto" onClick={exportCsv}>Export CSV</button>
      </div>
      <div className="max-h-[60vh] overflow-auto">
        <table className="min-w-full border-separate border-spacing-0 font-mono text-xs">
          <thead>
            <tr>
              <th className="sticky top-0 z-10 border-b border-slate-800 bg-slate-900 px-3 py-2 text-left text-slate-500">#</th>
              {columns.map((c) => (
                <th
                  key={c}
                  onClick={() => setSort((s) => ({ col: c, dir: s?.col === c && s.dir === 1 ? -1 : 1 }))}
                  className="sticky top-0 z-10 cursor-pointer select-none whitespace-nowrap border-b border-slate-800 bg-slate-900 px-3 py-2 text-left font-semibold text-slate-300 hover:text-white"
                >
                  {c}{sort?.col === c ? (sort.dir === 1 ? " ▲" : " ▼") : ""}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {view.slice(0, limit).map((r, i) => (
              <tr key={i} className="odd:bg-slate-900/40 hover:bg-sky-500/5">
                <td className="border-b border-slate-800/60 px-3 py-1.5 text-slate-600">{i + 1}</td>
                {columns.map((c) => (
                  <td key={c} className="max-w-[28rem] truncate border-b border-slate-800/60 px-3 py-1.5 text-slate-200" title={fmt(r[c])}>
                    {fmt(r[c])}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
        {view.length > limit && (
          <div className="p-3 text-center">
            <button className="btn-ghost" onClick={() => setLimit((l) => l + 500)}>Show more ({view.length - limit} remaining)</button>
          </div>
        )}
        {view.length === 0 && <div className="p-6 text-center text-sm text-slate-500">No rows match.</div>}
      </div>
    </div>
  );
}
