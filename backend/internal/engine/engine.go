// Package engine hosts the three detection modules:
//
//	A) Business Data Exception query runner (cron scheduled)
//	B) Zero-code runtime exception log watcher (watermark polling)
//	C) Infrastructure heartbeat: DB SELECT 1 + TCP/HTTP probes
//
// All work flows through one bounded WorkerPool; nothing blocks the HTTP server.
package engine

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/robfig/cron/v3"

	"sentinel/internal/config"
	"sentinel/internal/connectors"
	"sentinel/internal/incident"
	"sentinel/internal/store"
)

type Engine struct {
	cfg   config.Config
	store *store.Store
	conns *connectors.Pool
	inc   *incident.Service
	pool  *WorkerPool
	log   *slog.Logger
	http  *http.Client

	cron    *cron.Cron
	cronMu  sync.Mutex
	entries map[int64]cron.EntryID

	dueMu    sync.Mutex
	nextDue  map[string]time.Time
	failures map[string]int

	dirty atomic.Bool // set by the API when watchers / probes / data sources change
}

// ConfigChanged makes the supervisor reload watchers, probes and data sources on its
// next 1-second tick instead of waiting for the periodic 5-second refresh.
func (e *Engine) ConfigChanged() { e.dirty.Store(true) }

// CronParser accepts 5-field cron, optional seconds, and descriptors (@every 5m, @hourly).
var CronParser = cron.NewParser(cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

func ValidateSchedule(spec string) error {
	_, err := CronParser.Parse(spec)
	return err
}

func New(cfg config.Config, st *store.Store, conns *connectors.Pool, inc *incident.Service, log *slog.Logger) *Engine {
	return &Engine{
		cfg: cfg, store: st, conns: conns, inc: inc, log: log,
		pool:     NewWorkerPool(cfg.WorkerPoolSize, cfg.WorkerQueueSize, log),
		http:     &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
		cron:     cron.New(cron.WithParser(CronParser), cron.WithLocation(time.UTC)),
		entries:  map[int64]cron.EntryID{},
		nextDue:  map[string]time.Time{},
		failures: map[string]int{},
	}
}

// Start boots the worker pool, loads all rule schedules, starts the cron engine
// and the 1-second supervisor tick that drives watchers, heartbeats and probes.
func (e *Engine) Start(ctx context.Context) error {
	e.pool.Start(ctx, e.cfg.WorkerPoolSize)
	rules, err := e.store.ListRules(ctx)
	if err != nil {
		return err
	}
	for _, r := range rules {
		if err := e.ScheduleRule(r); err != nil {
			e.log.Warn("bad schedule, rule skipped", "rule", r.ID, "err", err)
		}
	}
	e.cron.Start()
	go e.supervise(ctx)
	e.log.Info("engine started", "rules", len(rules), "workers", e.cfg.WorkerPoolSize)
	return nil
}

func (e *Engine) Stop() {
	<-e.cron.Stop().Done()
	e.conns.CloseAll()
}

// ScheduleRule (re)registers a rule with the cron engine — called on CRUD for hot reload.
func (e *Engine) ScheduleRule(r store.Rule) error {
	e.cronMu.Lock()
	defer e.cronMu.Unlock()
	if id, ok := e.entries[r.ID]; ok {
		e.cron.Remove(id)
		delete(e.entries, r.ID)
	}
	if !r.Enabled {
		return nil
	}
	ruleID := r.ID
	id, err := e.cron.AddFunc(r.Schedule, func() { e.TriggerRule(ruleID) })
	if err != nil {
		return err
	}
	e.entries[r.ID] = id
	return nil
}

func (e *Engine) UnscheduleRule(id int64) {
	e.cronMu.Lock()
	defer e.cronMu.Unlock()
	if eid, ok := e.entries[id]; ok {
		e.cron.Remove(eid)
		delete(e.entries, id)
	}
}

// TriggerRule enqueues a rule run (cron tick or "Run now"). Non-blocking.
func (e *Engine) TriggerRule(id int64) bool {
	return e.pool.Submit("rule:"+strconv.FormatInt(id, 10), func(ctx context.Context) { e.runRule(ctx, id) })
}

// supervise ticks every second and dispatches due watchers / heartbeats / probes.
// Config is re-read every few seconds, so CRUD changes apply without restarts.
func (e *Engine) supervise(ctx context.Context) {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	var (
		watchers []store.LogWatcher
		probes   []store.Probe
		sources  []store.DataSource
		loadedAt time.Time
	)
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-tick.C:
			if e.dirty.Swap(false) || now.Sub(loadedAt) >= 5*time.Second {
				var err error
				if watchers, err = e.store.ListWatchers(ctx, true); err != nil {
					e.log.Error("load watchers", "err", err)
				}
				if probes, err = e.store.ListProbes(ctx, true); err != nil {
					e.log.Error("load probes", "err", err)
				}
				if sources, err = e.store.ListDataSources(ctx); err != nil {
					e.log.Error("load data sources", "err", err)
				}
				loadedAt = now
			}
			for _, w := range watchers {
				w := w
				e.ifDue(fmt.Sprintf("watch:%d", w.ID), time.Duration(w.IntervalSec)*time.Second, now, func(ctx context.Context) { e.runWatcher(ctx, w) })
			}
			for _, d := range sources {
				if d.HeartbeatEnabled {
					d := d
					e.ifDue(fmt.Sprintf("hb:%d", d.ID), e.cfg.HeartbeatInterval, now, func(ctx context.Context) { e.runDBHeartbeat(ctx, d) })
				}
			}
			for _, p := range probes {
				p := p
				e.ifDue(fmt.Sprintf("probe:%d", p.ID), time.Duration(p.IntervalSec)*time.Second, now, func(ctx context.Context) { e.runProbe(ctx, p) })
			}
		}
	}
}

func (e *Engine) ifDue(key string, every time.Duration, now time.Time, fn func(context.Context)) {
	e.dueMu.Lock()
	due, ok := e.nextDue[key]
	if ok && now.Before(due) {
		e.dueMu.Unlock()
		return
	}
	e.nextDue[key] = now.Add(every)
	e.dueMu.Unlock()
	e.pool.Submit(key, fn)
}

// consecutive-failure tracking for flap suppression
func (e *Engine) fail(key string) int {
	e.dueMu.Lock()
	defer e.dueMu.Unlock()
	e.failures[key]++
	return e.failures[key]
}

func (e *Engine) ok(key string) {
	e.dueMu.Lock()
	delete(e.failures, key)
	e.dueMu.Unlock()
}
