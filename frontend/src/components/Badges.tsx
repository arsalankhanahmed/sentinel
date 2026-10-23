import type { IncidentStatus, Severity } from "../types";

const sev: Record<Severity, string> = {
  critical: "bg-rose-500/15 text-rose-300 ring-rose-500/30",
  warning: "bg-amber-500/15 text-amber-300 ring-amber-500/30",
  info: "bg-sky-500/15 text-sky-300 ring-sky-500/30",
};
const st: Record<IncidentStatus, string> = {
  open: "bg-rose-500/10 text-rose-300",
  acknowledged: "bg-amber-500/10 text-amber-300",
  resolved: "bg-emerald-500/10 text-emerald-300",
};

export const SeverityBadge = ({ s }: { s: Severity }) => (
  <span className={`rounded px-1.5 py-0.5 text-[11px] font-semibold uppercase tracking-wide ring-1 ring-inset ${sev[s]}`}>{s}</span>
);

export const StatusBadge = ({ s }: { s: IncidentStatus }) => (
  <span className={`rounded px-1.5 py-0.5 text-[11px] font-medium capitalize ${st[s]}`}>{s}</span>
);

export const ago = (iso: string) => {
  const s = Math.max(0, (Date.now() - new Date(iso).getTime()) / 1000);
  if (s < 60) return `${Math.floor(s)}s ago`;
  if (s < 3600) return `${Math.floor(s / 60)}m ago`;
  if (s < 86400) return `${Math.floor(s / 3600)}h ago`;
  return `${Math.floor(s / 86400)}d ago`;
};
