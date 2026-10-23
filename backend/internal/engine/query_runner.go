package engine

import (
	"context"
	"fmt"
	"time"

	"sentinel/internal/connectors"
	"sentinel/internal/store"
)

// Module A — Business Data Exception Engine.
// Rule SQL returning Rows > 0  ⇒  business exception; rows captured as JSON in the Core DB.
// Rule SQL returning 0 rows    ⇒  any active incident for the rule auto-resolves.
func (e *Engine) runRule(ctx context.Context, ruleID int64) {
	start := time.Now()
	r, err := e.store.GetRule(ctx, ruleID)
	if err != nil {
		e.log.Error("rule load", "rule", ruleID, "err", err)
		return
	}
	if !r.Enabled {
		return
	}
	status := "ok"
	defer func() { e.store.MarkRuleRun(context.Background(), r.ID, status, time.Since(start)) }()

	ds, err := e.store.GetDataSource(ctx, r.DataSourceID)
	if err != nil {
		status = "error: data source: " + err.Error()
		return
	}
	db, err := e.conns.Get(ds)
	if err != nil {
		status = "error: connect: " + err.Error()
		return
	}
	qctx, cancel := context.WithTimeout(ctx, e.cfg.QueryTimeout)
	defer cancel()

	res, err := connectors.QueryReadOnly(qctx, db, r.SQLText, nil, r.MaxRows)
	if err != nil {
		status = "error: " + truncate(err.Error(), 400)
		e.log.Warn("rule query failed", "rule", r.Name, "err", err)
		return
	}

	fp := fmt.Sprintf("rule:%d", r.ID)
	if len(res.Rows) == 0 {
		e.inc.Resolve(ctx, fp)
		return
	}
	status = fmt.Sprintf("exception: %d rows", len(res.Rows))
	summary := fmt.Sprintf("Rule \"%s\" returned %d row(s) on %s.", r.Name, len(res.Rows), ds.Name)
	if res.Truncated {
		summary += fmt.Sprintf(" Capture truncated at %d rows.", r.MaxRows)
	}
	e.inc.Raise(ctx, store.NewIncident{
		Kind: "business_exception", SourceType: "rule", SourceID: r.ID, SourceName: ds.Name,
		Title: r.Name, Severity: r.Severity, Fingerprint: fp,
		Columns: res.Columns, Rows: res.Rows, Truncated: res.Truncated,
		Summary: summary, CorrectiveAction: r.CorrectiveAction,
	})
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
