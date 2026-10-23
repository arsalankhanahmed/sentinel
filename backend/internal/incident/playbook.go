package incident

import "strings"

// Playbook returns the dynamic "Recommended Corrective Action Plan" used when a
// rule/watcher has no static plan configured. Markdown-lite: one step per line.
func Playbook(kind, detail string) string {
	d := strings.ToLower(detail)
	switch kind {
	case "db_outage":
		steps := []string{
			"Confirm reachability from the platform host: TCP connect to the DB port (1433 / 5432 / 3306 / 1521).",
			"Check the database service state on the server (SQL Server service, postgres, mysqld, Oracle listener).",
			"Review the DB server's error log for crash, failover or out-of-disk events in the last 15 minutes.",
			"Verify the monitoring login is not locked or its password expired.",
			"If part of an AG / replica set, confirm which node is primary and that the listener points to it.",
			"Notify dependent application owners; incident auto-resolves once SELECT 1 succeeds again.",
		}
		if strings.Contains(d, "timeout") || strings.Contains(d, "deadline") {
			steps = append([]string{"Probe TIMED OUT (not refused): check for blocking/deadlocks, CPU saturation, or network/firewall drops first."}, steps...)
		}
		if strings.Contains(d, "login") || strings.Contains(d, "authentication") || strings.Contains(d, "password") {
			steps = append([]string{"Authentication failure detected: rotate / unlock the monitoring credential and update the data source."}, steps...)
		}
		return join(steps)
	case "endpoint_outage":
		steps := []string{
			"Check the host is up (ICMP / hypervisor console) and the service is listening on the port.",
			"For IIS: verify the App Pool is Started and not crash-looping (Event Viewer → System / Application).",
			"Inspect recent deployments, certificate expiry, and load-balancer health-check status.",
			"Check firewall / NSG changes between the platform and the target.",
			"Incident auto-resolves after the probe succeeds again.",
		}
		if strings.Contains(d, "503") {
			steps = append([]string{"HTTP 503: App Pool is likely stopped or rapid-fail protection tripped — recycle/start it."}, steps...)
		}
		if strings.Contains(d, "500") {
			steps = append([]string{"HTTP 500: application error — check app logs / runtime exception watcher for the root cause."}, steps...)
		}
		return join(steps)
	case "runtime_exception":
		return join([]string{
			"Open the captured rows below and group by error message / stack trace to find the dominant failure.",
			"Correlate timestamps with recent deployments, config changes and infrastructure incidents.",
			"Check whether the error is user-facing; if so, consider rollback or feature-flag disable.",
			"Create a defect ticket with the incident link; acknowledge here to stop duplicate escalation.",
		})
	default: // business_exception
		return join([]string{
			"Review the captured exception rows below; confirm they are genuine business exceptions, not a rule defect.",
			"Identify the upstream process / job that produced the records and check its last successful run.",
			"Correct the affected records through the owning application (not direct SQL) where possible.",
			"Acknowledge this incident; it auto-resolves when the rule next returns 0 rows.",
		})
	}
}

func join(s []string) string { return strings.Join(s, "\n") }
