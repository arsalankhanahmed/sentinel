import CrudPage from "../components/CrudPage";

export default function Probes() {
  return (
    <CrudPage
      title="Infrastructure Probes"
      intro="TCP handshake (e.g. SQL Server :1433, RDP :3389) or HTTP status check (IIS sites, health endpoints). Alerts after consecutive failures; auto-resolves on recovery."
      endpoint="/probes"
      fields={[
        { key: "name", label: "Name", required: true, placeholder: "IIS web01" },
        { key: "kind", label: "Probe type", type: "select", required: true, default: "http",
          options: [{ value: "http", label: "HTTP(S) status" }, { value: "tcp", label: "TCP handshake" }] },
        { key: "target", label: "Target", required: true, wide: true, placeholder: "https://web01.corp/health   or   db01.corp:1433" },
        { key: "expect_status", label: "Expected HTTP status", type: "number", default: 200 },
        { key: "timeout_ms", label: "Timeout (ms)", type: "number", default: 3000 },
        { key: "interval_sec", label: "Interval (sec)", type: "number", default: 60 },
        { key: "severity", label: "Severity", type: "select", default: "critical", options: [
          { value: "critical", label: "Critical" }, { value: "warning", label: "Warning" } ] },
      ]}
      columns={[
        { key: "name", label: "Name" },
        { key: "kind", label: "Type" },
        { key: "target", label: "Target" },
        { key: "interval_sec", label: "Every", render: (it) => `${it.interval_sec}s` },
      ]}
    />
  );
}
