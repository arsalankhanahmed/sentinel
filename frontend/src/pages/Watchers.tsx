import CrudPage from "../components/CrudPage";
import { useFetch } from "../hooks/useFetch";
import type { DataSource } from "../types";

export default function Watchers() {
  const ds = useFetch<DataSource[]>("/datasources");
  return (
    <CrudPage
      title="Runtime Log Watchers"
      intro="Zero-code: no SDK in your apps. Sentinel polls the app's own error-log table using an identity-column watermark and alerts only on NEW rows."
      endpoint="/watchers"
      fields={[
        { key: "name", label: "Name", required: true, placeholder: "Order API errors" },
        { key: "data_source_id", label: "Data source", type: "select", required: true,
          options: (ds.data ?? []).map((d) => ({ value: String(d.id), label: d.name })) },
        { key: "table_name", label: "Log table", required: true, placeholder: "dbo.ErrorLog" },
        { key: "id_column", label: "Identity column", required: true, placeholder: "Id" },
        { key: "select_columns", label: "Columns", default: "*", placeholder: "* or Level, Message, StackTrace, LoggedAt" },
        { key: "filter_sql", label: "Filter (optional)", placeholder: "Level IN ('Error','Fatal')" },
        { key: "interval_sec", label: "Poll interval (sec)", type: "number", default: 30 },
        { key: "severity", label: "Severity", type: "select", default: "critical", options: [
          { value: "critical", label: "Critical" }, { value: "warning", label: "Warning" }, { value: "info", label: "Info" } ] },
        { key: "corrective_action", label: "Corrective action plan (one step per line, optional)", type: "textarea" },
      ]}
      transform={(v) => ({ ...v, data_source_id: Number(v.data_source_id) })}
      columns={[
        { key: "name", label: "Name" },
        { key: "table_name", label: "Table" },
        { key: "interval_sec", label: "Every", render: (it) => `${it.interval_sec}s` },
        { key: "cursor_value", label: "Watermark", render: (it) => (Number(it.cursor_value) < 0 ? "initialising" : String(it.cursor_value)) },
      ]}
    />
  );
}
