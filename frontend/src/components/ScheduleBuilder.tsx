import { useEffect, useState } from "react";

type Mode = "minutes" | "hourly" | "daily" | "weekly" | "cron";
const DAYS = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];

/** UI selectors → cron spec. Emits a standard 5-field cron (or raw custom cron). */
export function toCron(m: Mode, s: { every: number; minute: number; hour: number; dow: number; raw: string }) {
  switch (m) {
    case "minutes": return `*/${Math.max(1, s.every)} * * * *`;
    case "hourly": return `${s.minute} * * * *`;
    case "daily": return `${s.minute} ${s.hour} * * *`;
    case "weekly": return `${s.minute} ${s.hour} * * ${s.dow}`;
    default: return s.raw;
  }
}

export default function ScheduleBuilder({ value, onChange }: { value: string; onChange: (v: string) => void }) {
  const [mode, setMode] = useState<Mode>(value && !value.startsWith("*/") ? "cron" : "minutes");
  const [s, setS] = useState({ every: 5, minute: 0, hour: 8, dow: 1, raw: value || "*/5 * * * *" });

  useEffect(() => { onChange(toCron(mode, s)); }, [mode, s]); // eslint-disable-line react-hooks/exhaustive-deps

  const num = (k: keyof typeof s, min: number, max: number) => (
    <input type="number" min={min} max={max} className="input w-20" value={s[k] as number}
      onChange={(e) => setS({ ...s, [k]: Math.min(max, Math.max(min, Number(e.target.value))) })} />
  );

  return (
    <div className="space-y-2">
      <div className="flex flex-wrap gap-1">
        {(["minutes", "hourly", "daily", "weekly", "cron"] as Mode[]).map((m) => (
          <button type="button" key={m} onClick={() => setMode(m)}
            className={`btn capitalize ${mode === m ? "bg-sky-600 text-white" : "border border-slate-700 text-slate-300"}`}>
            {m === "cron" ? "Custom cron" : m}
          </button>
        ))}
      </div>
      <div className="flex flex-wrap items-center gap-2 text-sm text-slate-300">
        {mode === "minutes" && <>Every {num("every", 1, 59)} minutes</>}
        {mode === "hourly" && <>Every hour at minute {num("minute", 0, 59)}</>}
        {(mode === "daily" || mode === "weekly") && (
          <>
            {mode === "weekly" && (
              <select className="input w-28" value={s.dow} onChange={(e) => setS({ ...s, dow: Number(e.target.value) })}>
                {DAYS.map((d, i) => <option key={d} value={i}>{d}</option>)}
              </select>
            )}
            at {num("hour", 0, 23)} : {num("minute", 0, 59)} UTC
          </>
        )}
        {mode === "cron" && (
          <input className="input font-mono" value={s.raw} onChange={(e) => setS({ ...s, raw: e.target.value })}
            placeholder="*/10 * * * *   or   @every 30s" />
        )}
      </div>
      <code className="text-xs text-slate-500">{toCron(mode, s)}</code>
    </div>
  );
}
