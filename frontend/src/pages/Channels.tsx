import CrudPage from "../components/CrudPage";
import { post } from "../api/client";

const EXAMPLES = `Teams:    {"webhook_url":"https://prod-xx.westeurope.logic.azure.com/workflows/..."}
Zoom:     {"webhook_url":"https://integrations.zoom.us/chat/webhooks/incomingwebhook/...","verification_token":"..."}
WhatsApp: {"provider":"meta","phone_number_id":"...","access_token":"...","template":"incident_alert","recipients":["923001234567"]}
          {"provider":"twilio","account_sid":"AC...","auth_token":"...","from":"whatsapp:+1415...","recipients":["whatsapp:+92300..."]}
Email:    {"to":["oncall@company.com"]}`;

export default function Channels() {
  return (
    <CrudPage
      title="Notification Channels"
      intro="Notifications carry only a summary and a secure deep link — never the captured rows. Channel secrets are encrypted at rest."
      endpoint="/channels"
      fields={[
        { key: "name", label: "Name", required: true, placeholder: "Ops Teams channel" },
        { key: "kind", label: "Channel", type: "select", required: true, options: [
          { value: "teams", label: "Microsoft Teams (Adaptive Card)" }, { value: "zoom", label: "Zoom Team Chat" },
          { value: "whatsapp", label: "WhatsApp (Meta / Twilio)" }, { value: "email", label: "Email (HTML)" } ] },
        { key: "min_severity", label: "Minimum severity", type: "select", default: "warning", options: [
          { value: "info", label: "Info" }, { value: "warning", label: "Warning" }, { value: "critical", label: "Critical" } ] },
        { key: "config", label: "Config (JSON)", type: "json", required: true, placeholder: EXAMPLES },
      ]}
      columns={[
        { key: "name", label: "Name" },
        { key: "kind", label: "Channel" },
        { key: "min_severity", label: "Min severity" },
      ]}
      rowAction={{
        label: "Send test",
        run: async (it) => {
          const r = await post<{ ok: boolean; error?: string }>(`/channels/${it.id}/test`);
          return r.ok ? "✓ delivered" : `✗ ${r.error}`;
        },
      }}
    />
  );
}
