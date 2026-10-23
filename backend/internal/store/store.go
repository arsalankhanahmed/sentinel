// Package store is the Core DB repository (PostgreSQL via pgx — fastest Go PG driver).
package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"sentinel/migrations"
)

var ErrNotFound = errors.New("not found")

type Store struct{ db *pgxpool.Pool }

func Open(ctx context.Context, url string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, err
	}
	cfg.MaxConns = 32
	cfg.MinConns = 2
	cfg.MaxConnIdleTime = 5 * time.Minute
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, err
	}
	return &Store{db: pool}, nil
}

func (s *Store) Close()                         { s.db.Close() }
func (s *Store) Ping(ctx context.Context) error { return s.db.Ping(ctx) }
func (s *Store) Migrate(ctx context.Context) error {
	_, err := s.db.Exec(ctx, migrations.InitSQL)
	return err
}

func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

// ---------------------------------------------------------------- data sources

const dsCols = `id, name, driver, dsn_enc, heartbeat_enabled, created_at, updated_at`

func scanDS(r pgx.Row) (DataSource, error) {
	var d DataSource
	err := r.Scan(&d.ID, &d.Name, &d.Driver, &d.DSNEnc, &d.HeartbeatEnabled, &d.CreatedAt, &d.UpdatedAt)
	return d, notFound(err)
}

func (s *Store) ListDataSources(ctx context.Context) ([]DataSource, error) {
	rows, err := s.db.Query(ctx, `SELECT `+dsCols+` FROM data_sources ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []DataSource{}
	for rows.Next() {
		d, err := scanDS(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) GetDataSource(ctx context.Context, id int64) (DataSource, error) {
	return scanDS(s.db.QueryRow(ctx, `SELECT `+dsCols+` FROM data_sources WHERE id=$1`, id))
}

func (s *Store) CreateDataSource(ctx context.Context, d DataSource) (int64, error) {
	var id int64
	err := s.db.QueryRow(ctx,
		`INSERT INTO data_sources (name, driver, dsn_enc, heartbeat_enabled) VALUES ($1,$2,$3,$4) RETURNING id`,
		d.Name, d.Driver, d.DSNEnc, d.HeartbeatEnabled).Scan(&id)
	return id, err
}

func (s *Store) DeleteDataSource(ctx context.Context, id int64) error {
	_, err := s.db.Exec(ctx, `DELETE FROM data_sources WHERE id=$1`, id)
	return err
}

// ---------------------------------------------------------------- rules

const ruleCols = `id, name, data_source_id, sql_text, schedule, severity, corrective_action, max_rows, enabled, last_run_at, last_status, last_duration_ms`

func scanRule(r pgx.Row) (Rule, error) {
	var x Rule
	err := r.Scan(&x.ID, &x.Name, &x.DataSourceID, &x.SQLText, &x.Schedule, &x.Severity, &x.CorrectiveAction,
		&x.MaxRows, &x.Enabled, &x.LastRunAt, &x.LastStatus, &x.LastDurationMs)
	return x, notFound(err)
}

func (s *Store) ListRules(ctx context.Context) ([]Rule, error) {
	rows, err := s.db.Query(ctx, `SELECT `+ruleCols+` FROM exception_rules ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Rule{}
	for rows.Next() {
		x, err := scanRule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (s *Store) GetRule(ctx context.Context, id int64) (Rule, error) {
	return scanRule(s.db.QueryRow(ctx, `SELECT `+ruleCols+` FROM exception_rules WHERE id=$1`, id))
}

func (s *Store) CreateRule(ctx context.Context, x Rule) (int64, error) {
	var id int64
	err := s.db.QueryRow(ctx, `INSERT INTO exception_rules
		(name, data_source_id, sql_text, schedule, severity, corrective_action, max_rows, enabled)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`,
		x.Name, x.DataSourceID, x.SQLText, x.Schedule, x.Severity, x.CorrectiveAction, x.MaxRows, x.Enabled).Scan(&id)
	return id, err
}

func (s *Store) UpdateRule(ctx context.Context, x Rule) error {
	ct, err := s.db.Exec(ctx, `UPDATE exception_rules SET name=$2, data_source_id=$3, sql_text=$4, schedule=$5,
		severity=$6, corrective_action=$7, max_rows=$8, enabled=$9, updated_at=now() WHERE id=$1`,
		x.ID, x.Name, x.DataSourceID, x.SQLText, x.Schedule, x.Severity, x.CorrectiveAction, x.MaxRows, x.Enabled)
	if err == nil && ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func (s *Store) DeleteRule(ctx context.Context, id int64) error {
	_, err := s.db.Exec(ctx, `DELETE FROM exception_rules WHERE id=$1`, id)
	return err
}

func (s *Store) MarkRuleRun(ctx context.Context, id int64, status string, dur time.Duration) {
	_, _ = s.db.Exec(ctx, `UPDATE exception_rules SET last_run_at=now(), last_status=$2, last_duration_ms=$3 WHERE id=$1`,
		id, status, int(dur.Milliseconds()))
}

// ---------------------------------------------------------------- log watchers

const watchCols = `id, name, data_source_id, table_name, id_column, select_columns, filter_sql, interval_sec, batch_limit, cursor_value, severity, corrective_action, enabled`

func (s *Store) ListWatchers(ctx context.Context, onlyEnabled bool) ([]LogWatcher, error) {
	q := `SELECT ` + watchCols + ` FROM log_watchers`
	if onlyEnabled {
		q += ` WHERE enabled`
	}
	rows, err := s.db.Query(ctx, q+` ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LogWatcher{}
	for rows.Next() {
		var w LogWatcher
		if err := rows.Scan(&w.ID, &w.Name, &w.DataSourceID, &w.TableName, &w.IDColumn, &w.SelectColumns, &w.FilterSQL,
			&w.IntervalSec, &w.BatchLimit, &w.CursorValue, &w.Severity, &w.CorrectiveAction, &w.Enabled); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (s *Store) CreateWatcher(ctx context.Context, w LogWatcher) (int64, error) {
	var id int64
	err := s.db.QueryRow(ctx, `INSERT INTO log_watchers
		(name, data_source_id, table_name, id_column, select_columns, filter_sql, interval_sec, batch_limit, severity, corrective_action, enabled)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING id`,
		w.Name, w.DataSourceID, w.TableName, w.IDColumn, w.SelectColumns, w.FilterSQL, w.IntervalSec, w.BatchLimit,
		w.Severity, w.CorrectiveAction, w.Enabled).Scan(&id)
	return id, err
}

func (s *Store) DeleteWatcher(ctx context.Context, id int64) error {
	_, err := s.db.Exec(ctx, `DELETE FROM log_watchers WHERE id=$1`, id)
	return err
}

// AdvanceCursor is monotonic: it never moves the watermark backwards.
func (s *Store) AdvanceCursor(ctx context.Context, id, v int64) error {
	_, err := s.db.Exec(ctx, `UPDATE log_watchers SET cursor_value=$2 WHERE id=$1 AND cursor_value < $2`, id, v)
	return err
}

// ---------------------------------------------------------------- probes

func (s *Store) ListProbes(ctx context.Context, onlyEnabled bool) ([]Probe, error) {
	q := `SELECT id, name, kind, target, expect_status, timeout_ms, interval_sec, severity, enabled FROM probes`
	if onlyEnabled {
		q += ` WHERE enabled`
	}
	rows, err := s.db.Query(ctx, q+` ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Probe{}
	for rows.Next() {
		var p Probe
		if err := rows.Scan(&p.ID, &p.Name, &p.Kind, &p.Target, &p.ExpectStatus, &p.TimeoutMs, &p.IntervalSec, &p.Severity, &p.Enabled); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) CreateProbe(ctx context.Context, p Probe) (int64, error) {
	var id int64
	err := s.db.QueryRow(ctx, `INSERT INTO probes (name, kind, target, expect_status, timeout_ms, interval_sec, severity, enabled)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`,
		p.Name, p.Kind, p.Target, p.ExpectStatus, p.TimeoutMs, p.IntervalSec, p.Severity, p.Enabled).Scan(&id)
	return id, err
}

func (s *Store) DeleteProbe(ctx context.Context, id int64) error {
	_, err := s.db.Exec(ctx, `DELETE FROM probes WHERE id=$1`, id)
	return err
}

// ---------------------------------------------------------------- channels

func (s *Store) ListChannels(ctx context.Context, onlyEnabled bool) ([]Channel, error) {
	q := `SELECT id, name, kind, config_enc, min_severity, enabled FROM notification_channels`
	if onlyEnabled {
		q += ` WHERE enabled`
	}
	rows, err := s.db.Query(ctx, q+` ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Channel{}
	for rows.Next() {
		var c Channel
		if err := rows.Scan(&c.ID, &c.Name, &c.Kind, &c.ConfigEnc, &c.MinSeverity, &c.Enabled); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) CreateChannel(ctx context.Context, c Channel) (int64, error) {
	var id int64
	err := s.db.QueryRow(ctx, `INSERT INTO notification_channels (name, kind, config_enc, min_severity, enabled)
		VALUES ($1,$2,$3,$4,$5) RETURNING id`, c.Name, c.Kind, c.ConfigEnc, c.MinSeverity, c.Enabled).Scan(&id)
	return id, err
}

func (s *Store) DeleteChannel(ctx context.Context, id int64) error {
	_, err := s.db.Exec(ctx, `DELETE FROM notification_channels WHERE id=$1`, id)
	return err
}

func (s *Store) LogNotification(ctx context.Context, incidentID string, channelID int64, event, status string, attempts int, errMsg string) {
	_, _ = s.db.Exec(ctx, `INSERT INTO notification_log (incident_id, channel_id, event, status, attempts, error)
		VALUES ($1,$2,$3,$4,$5,NULLIF($6,''))`, incidentID, channelID, event, status, attempts, errMsg)
}

// ---------------------------------------------------------------- incidents

// UpsertIncident inserts a new active incident, or — if one is already active
// for the same fingerprint — refreshes it in place. inserted=true means a NEW
// incident was opened and notifications should fire.
func (s *Store) UpsertIncident(ctx context.Context, n NewIncident) (id string, inserted bool, err error) {
	cols, _ := json.Marshal(n.Columns)
	if n.Rows == nil {
		n.Rows = []map[string]any{}
	}
	payload, err := json.Marshal(n.Rows)
	if err != nil {
		return "", false, err
	}
	err = s.db.QueryRow(ctx, `
		INSERT INTO incidents (kind, source_type, source_id, source_name, title, severity, fingerprint,
		                       row_count, truncated, columns, payload, summary, corrective_action,
		                       link_token_hash, link_expires_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		ON CONFLICT (fingerprint) WHERE status <> 'resolved'
		DO UPDATE SET row_count = EXCLUDED.row_count, truncated = EXCLUDED.truncated,
		              columns = EXCLUDED.columns, payload = EXCLUDED.payload, summary = EXCLUDED.summary,
		              last_seen_at = now(), occurrences = incidents.occurrences + 1
		RETURNING id::text, (xmax = 0)`,
		n.Kind, n.SourceType, n.SourceID, n.SourceName, n.Title, n.Severity, n.Fingerprint,
		len(n.Rows), n.Truncated, cols, payload, n.Summary, n.CorrectiveAction,
		n.LinkTokenHash, n.LinkExpiresAt).Scan(&id, &inserted)
	return id, inserted, err
}

// ResolveActive closes the active incident for a fingerprint (auto-recovery).
func (s *Store) ResolveActive(ctx context.Context, fingerprint string) (*Incident, error) {
	inc, err := scanIncident(s.db.QueryRow(ctx, `UPDATE incidents SET status='resolved', resolved_at=now()
		WHERE fingerprint=$1 AND status <> 'resolved' RETURNING `+incListCols, fingerprint), false)
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	return inc, err
}

const incListCols = `id::text, kind, source_type, source_id, source_name, title, severity, status, row_count, truncated,
	summary, corrective_action, occurrences, opened_at, last_seen_at, acknowledged_by, resolved_at, link_expires_at`

func scanIncident(r pgx.Row, full bool) (*Incident, error) {
	var i Incident
	dst := []any{&i.ID, &i.Kind, &i.SourceType, &i.SourceID, &i.SourceName, &i.Title, &i.Severity, &i.Status,
		&i.RowCount, &i.Truncated, &i.Summary, &i.CorrectiveAction, &i.Occurrences, &i.OpenedAt, &i.LastSeenAt,
		&i.AcknowledgedBy, &i.ResolvedAt, &i.LinkExpiresAt}
	if full {
		dst = append(dst, &i.Columns, &i.Payload)
	}
	if err := r.Scan(dst...); err != nil {
		return nil, notFound(err)
	}
	return &i, nil
}

// ListIncidents returns metadata only — never the raw payload.
func (s *Store) ListIncidents(ctx context.Context, status string, limit int) ([]Incident, error) {
	q := `SELECT ` + incListCols + ` FROM incidents`
	args := []any{limit}
	if status != "" {
		q += ` WHERE status = $2`
		args = append(args, status)
	}
	rows, err := s.db.Query(ctx, q+` ORDER BY opened_at DESC LIMIT $1`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Incident{}
	for rows.Next() {
		i, err := scanIncident(rows, false)
		if err != nil {
			return nil, err
		}
		out = append(out, *i)
	}
	return out, rows.Err()
}

// GetIncidentByLink resolves a deep-link token hash to the full incident incl. captured rows.
func (s *Store) GetIncidentByLink(ctx context.Context, tokenHash []byte) (*Incident, error) {
	return scanIncident(s.db.QueryRow(ctx, `SELECT `+incListCols+`, columns, payload FROM incidents
		WHERE link_token_hash=$1 AND link_expires_at > now()`, tokenHash), true)
}

func (s *Store) SetIncidentStatus(ctx context.Context, id, status, actor string) error {
	var q string
	switch status {
	case "acknowledged":
		q = `UPDATE incidents SET status='acknowledged', acknowledged_by=$2, acknowledged_at=now() WHERE id=$1 AND status='open'`
	case "resolved":
		q = `UPDATE incidents SET status='resolved', resolved_at=now(), acknowledged_by=COALESCE(acknowledged_by,$2) WHERE id=$1 AND status<>'resolved'`
	default:
		return errors.New("invalid status")
	}
	ct, err := s.db.Exec(ctx, q, id, actor)
	if err == nil && ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func (s *Store) Stats(ctx context.Context) (Stats, error) {
	st := Stats{OpenByKind: map[string]int{}, OpenBySeverity: map[string]int{}}
	rows, err := s.db.Query(ctx, `SELECT kind, severity, count(*) FROM incidents WHERE status<>'resolved' GROUP BY kind, severity`)
	if err != nil {
		return st, err
	}
	for rows.Next() {
		var k, sev string
		var n int
		if err := rows.Scan(&k, &sev, &n); err != nil {
			rows.Close()
			return st, err
		}
		st.OpenByKind[k] += n
		st.OpenBySeverity[sev] += n
	}
	rows.Close()
	err = s.db.QueryRow(ctx, `SELECT count(*) FROM incidents WHERE opened_at > now() - interval '24 hours'`).Scan(&st.Last24h)
	return st, err
}

func (s *Store) Audit(ctx context.Context, actor, action, target, ip string) {
	_, _ = s.db.Exec(ctx, `INSERT INTO audit_log (actor, action, target, ip) VALUES ($1,$2,$3,$4)`, actor, action, target, ip)
}

// Exists reports whether a config row still exists (guards against acting on a
// source that was deleted while a check was in flight). Table names are fixed.
func (s *Store) Exists(ctx context.Context, table string, id int64) bool {
	q, ok := map[string]string{
		"data_sources": `SELECT EXISTS (SELECT 1 FROM data_sources WHERE id=$1)`,
		"probes":       `SELECT EXISTS (SELECT 1 FROM probes WHERE id=$1 AND enabled)`,
		"log_watchers": `SELECT EXISTS (SELECT 1 FROM log_watchers WHERE id=$1 AND enabled)`,
	}[table]
	if !ok {
		return false
	}
	var ex bool
	if err := s.db.QueryRow(ctx, q, id).Scan(&ex); err != nil {
		return true // fail open: never suppress a real alert because of a lookup error
	}
	return ex
}
