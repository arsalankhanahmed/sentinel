package engine

import (
	"context"
	"fmt"
	"strings"

	"sentinel/internal/connectors"
	"sentinel/internal/store"
)

// Module B — Zero-code Runtime Application Exception watcher.
// Polls an application's own error/log table using a monotonic identity-column
// watermark: only NEW rows since the last poll are read (index seek, O(new rows)).
// No SDK, agent, or code change is required in the target application.
func (e *Engine) runWatcher(ctx context.Context, w store.LogWatcher) {
	ds, err := e.store.GetDataSource(ctx, w.DataSourceID)
	if err != nil {
		e.log.Error("watcher data source", "watcher", w.ID, "err", err)
		return
	}
	db, err := e.conns.Get(ds)
	if err != nil {
		e.log.Warn("watcher connect", "watcher", w.Name, "err", err)
		return
	}
	qctx, cancel := context.WithTimeout(ctx, e.cfg.QueryTimeout)
	defer cancel()

	// First run: start from the current high-water mark (don't replay history).
	if w.CursorValue < 0 {
		q, err := connectors.MaxIDQuery(w.TableName, w.IDColumn)
		if err != nil {
			e.log.Error("watcher definition", "watcher", w.Name, "err", err)
			return
		}
		var max int64
		if err := db.QueryRowContext(qctx, q).Scan(&max); err != nil {
			e.log.Warn("watcher init", "watcher", w.Name, "err", err)
			return
		}
		_ = e.store.AdvanceCursor(ctx, w.ID, max)
		e.log.Info("watcher initialised", "watcher", w.Name, "cursor", max)
		return
	}

	q, err := connectors.WatcherQuery(ds.Driver, w.TableName, w.IDColumn, w.SelectColumns, w.FilterSQL, w.BatchLimit)
	if err != nil {
		e.log.Error("watcher definition", "watcher", w.Name, "err", err)
		return
	}
	res, err := connectors.QueryReadOnly(qctx, db, q, []any{w.CursorValue}, w.BatchLimit)
	if err != nil {
		e.log.Warn("watcher query", "watcher", w.Name, "err", err)
		return
	}
	if len(res.Rows) == 0 {
		return
	}

	// Advance the watermark to the max id seen in this batch.
	maxID := w.CursorValue
	for _, row := range res.Rows {
		if v, err := connectors.AsInt64(lookup(row, w.IDColumn)); err == nil && v > maxID {
			maxID = v
		}
	}
	if maxID <= w.CursorValue {
		e.log.Error("watcher could not read id column; check id_column", "watcher", w.Name)
		return
	}

	if !e.store.Exists(ctx, "log_watchers", w.ID) {
		return
	}
	// One incident per batch; the fingerprint includes the batch watermark so every
	// new burst of errors is a distinct, linkable incident.
	e.inc.Raise(ctx, store.NewIncident{
		Kind: "runtime_exception", SourceType: "watcher", SourceID: w.ID, SourceName: ds.Name,
		Title: w.Name, Severity: w.Severity, Fingerprint: fmt.Sprintf("watch:%d:%d", w.ID, maxID),
		Columns: res.Columns, Rows: res.Rows, Truncated: res.Truncated,
		Summary:          fmt.Sprintf("%d new error row(s) detected in %s on %s.", len(res.Rows), w.TableName, ds.Name),
		CorrectiveAction: w.CorrectiveAction,
	})
	if err := e.store.AdvanceCursor(ctx, w.ID, maxID); err != nil {
		e.log.Error("advance cursor", "watcher", w.Name, "err", err)
	}
}

// lookup tolerates quoted/bracketed id column names ([Id], "Id", schema-less).
func lookup(row map[string]any, col string) any {
	if v, ok := row[col]; ok {
		return v
	}
	clean := trimQuotes(col)
	for k, v := range row {
		if equalFold(k, clean) {
			return v
		}
	}
	return nil
}

func trimQuotes(s string) string {
	if i := strings.LastIndex(s, "."); i >= 0 {
		s = s[i+1:]
	}
	return strings.Trim(s, "[]\"`")
}

func equalFold(a, b string) bool { return strings.EqualFold(a, b) }
