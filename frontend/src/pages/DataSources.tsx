import CrudPage from "../components/CrudPage";
import { post } from "../api/client";

const DSN_HINTS: Record<string, string> = {
  sqlserver: "sqlserver://monitor:****@db01:1433?database=ERP&encrypt=true",
  postgres: "postgres://monitor:****@pg01:5432/app?sslmode=require",
  mysql: "monitor:****@tcp(mysql01:3306)/app?parseTime=true&tls=true",
  oracle: "oracle://monitor:****@ora01:1521/ORCLPDB1",
};

export default function DataSources() {
  return (
    <CrudPage
      title="Data Sources"
      intro="Connection strings are encrypted with AES-256-GCM before storage and are never returned by the API. Use a least-privilege, read-only login."
      endpoint="/datasources"
      fields={[
        { key: "name", label: "Name", required: true, placeholder: "ERP Production" },
        { key: "driver", label: "Engine", type: "select", required: true, options: [
          { value: "sqlserver", label: "SQL Server" }, { value: "postgres", label: "PostgreSQL" },
          { value: "mysql", label: "MySQL" }, { value: "oracle", label: "Oracle" } ] },
        { key: "dsn", label: "Connection string", type: "password", required: true, wide: true,
          hint: Object.entries(DSN_HINTS).map(([k, v]) => `${k}: ${v}`).join("  ·  ") },
        { key: "heartbeat_enabled", label: "Outage heartbeat (SELECT 1 every 60s)", type: "checkbox", default: true },
      ]}
      columns={[
        { key: "name", label: "Name" },
        { key: "driver", label: "Engine" },
        { key: "heartbeat_enabled", label: "Heartbeat", render: (it) => (it.heartbeat_enabled ? "On" : "Off") },
      ]}
      rowAction={{
        label: "Test",
        run: async (it) => {
          const r = await post<{ ok: boolean; latency_ms?: number; error?: string }>(`/datasources/${it.id}/test`);
          return r.ok ? `✓ ${r.latency_ms} ms` : `✗ ${r.error}`;
        },
      }}
    />
  );
}
