package engine

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"sentinel/internal/connectors"
	"sentinel/internal/store"
)

// Module C — Infrastructure & DB outage monitor.

// runDBHeartbeat executes SELECT 1 (SELECT 1 FROM DUAL on Oracle) with a hard timeout.
func (e *Engine) runDBHeartbeat(ctx context.Context, d store.DataSource) {
	key := fmt.Sprintf("ds:%d", d.ID)
	err := func() error {
		db, err := e.conns.Get(d)
		if err != nil {
			return err
		}
		pctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		return connectors.Ping(pctx, db, d.Driver)
	}()
	if err == nil {
		e.ok(key)
		e.inc.Resolve(ctx, key)
		return
	}
	// Drop pooled connections so the next probe dials fresh (avoid stale sockets).
	e.conns.Invalidate(d.ID)
	if n := e.fail(key); n < e.cfg.ProbeFailThreshold {
		e.log.Warn("db heartbeat failed (below threshold)", "source", d.Name, "n", n, "err", err)
		return
	}
	if !e.store.Exists(ctx, "data_sources", d.ID) { // deleted while the check was in flight
		return
	}
	e.inc.Raise(ctx, store.NewIncident{
		Kind: "db_outage", SourceType: "datasource", SourceID: d.ID, SourceName: d.Name,
		Title: "Database unreachable: " + d.Name, Severity: "critical", Fingerprint: key,
		Columns: []string{"data_source", "driver", "error", "checked_at"},
		Rows: []map[string]any{{"data_source": d.Name, "driver": d.Driver, "error": err.Error(),
			"checked_at": time.Now().UTC().Format(time.RFC3339)}},
		Summary: "Heartbeat failed: " + truncate(err.Error(), 200),
	})
}

// runProbe performs a TCP handshake or HTTP(S) status check against a server/IIS endpoint.
func (e *Engine) runProbe(ctx context.Context, p store.Probe) {
	key := fmt.Sprintf("probe:%d", p.ID)
	timeout := time.Duration(p.TimeoutMs) * time.Millisecond
	start := time.Now()
	var (
		err    error
		status int
	)
	switch p.Kind {
	case "tcp":
		var c net.Conn
		d := net.Dialer{Timeout: timeout}
		c, err = d.DialContext(ctx, "tcp", p.Target)
		if err == nil {
			_ = c.Close()
		}
	case "http":
		pctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		req, rerr := http.NewRequestWithContext(pctx, http.MethodGet, p.Target, nil)
		if rerr != nil {
			err = rerr
			break
		}
		req.Header.Set("User-Agent", "Sentinel-Probe/1.0")
		resp, herr := e.http.Do(req)
		if herr != nil {
			err = herr
			break
		}
		status = resp.StatusCode
		resp.Body.Close()
		if status != p.ExpectStatus {
			err = fmt.Errorf("unexpected HTTP %d (expected %d)", status, p.ExpectStatus)
		}
	default:
		err = fmt.Errorf("unknown probe kind %q", p.Kind)
	}
	latency := time.Since(start)

	if err == nil {
		e.ok(key)
		e.inc.Resolve(ctx, key)
		return
	}
	if n := e.fail(key); n < e.cfg.ProbeFailThreshold {
		e.log.Warn("probe failed (below threshold)", "probe", p.Name, "n", n, "err", err)
		return
	}
	if !e.store.Exists(ctx, "probes", p.ID) { // deleted while the check was in flight
		return
	}
	e.inc.Raise(ctx, store.NewIncident{
		Kind: "endpoint_outage", SourceType: "probe", SourceID: p.ID, SourceName: p.Target,
		Title: "Endpoint down: " + p.Name, Severity: p.Severity, Fingerprint: key,
		Columns: []string{"probe", "kind", "target", "http_status", "latency_ms", "error", "checked_at"},
		Rows: []map[string]any{{"probe": p.Name, "kind": p.Kind, "target": p.Target, "http_status": status,
			"latency_ms": latency.Milliseconds(), "error": err.Error(), "checked_at": time.Now().UTC().Format(time.RFC3339)}},
		Summary: fmt.Sprintf("%s probe to %s failed: %s", p.Kind, p.Target, truncate(err.Error(), 200)),
	})
}
