export type Severity = "info" | "warning" | "critical";
export type IncidentKind = "business_exception" | "runtime_exception" | "db_outage" | "endpoint_outage";
export type IncidentStatus = "open" | "acknowledged" | "resolved";

export interface Incident {
  id: string;
  kind: IncidentKind;
  source_type: string;
  source_id: number;
  source_name: string;
  title: string;
  severity: Severity;
  status: IncidentStatus;
  row_count: number;
  truncated: boolean;
  columns?: string[];
  payload?: Record<string, unknown>[];
  summary: string;
  corrective_action: string;
  occurrences: number;
  opened_at: string;
  last_seen_at: string;
  acknowledged_by: string | null;
  resolved_at: string | null;
  link_expires_at: string;
}

export interface Stats {
  open_by_kind: Partial<Record<IncidentKind, number>>;
  open_by_severity: Partial<Record<Severity, number>>;
  last_24h: number;
}

export interface DataSource {
  id: number;
  name: string;
  driver: "sqlserver" | "postgres" | "mysql" | "oracle";
  heartbeat_enabled: boolean;
  created_at: string;
}

export interface Rule {
  id: number;
  name: string;
  data_source_id: number;
  sql_text: string;
  schedule: string;
  severity: Severity;
  corrective_action: string;
  max_rows: number;
  enabled: boolean;
  last_run_at: string | null;
  last_status: string | null;
  last_duration_ms: number | null;
}

export const KIND_LABEL: Record<IncidentKind, string> = {
  business_exception: "Business Exception",
  runtime_exception: "Runtime Exception",
  db_outage: "DB Outage",
  endpoint_outage: "Endpoint Outage",
};
