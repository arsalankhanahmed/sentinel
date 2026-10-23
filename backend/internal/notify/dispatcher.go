// Package notify fans incident events out to Teams, Zoom, WhatsApp and Email.
// Payloads carry ONLY metadata + a deep link; captured rows never leave the Core DB.
package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"sentinel/internal/store"
	"sentinel/internal/vault"
)

// Message is the compliance-safe envelope sent to every channel.
type Message struct {
	Event      string // opened | resolved
	IncidentID string
	Title      string
	Kind       string
	Source     string
	Severity   string
	RowCount   int
	Summary    string // short, human summary — no raw data
	Timestamp  time.Time
	Link       string // deep link (empty on resolved events)
}

type Adapter interface {
	Send(ctx context.Context, cfg json.RawMessage, m Message) error
}

type job struct {
	ch  store.Channel
	cfg json.RawMessage
	msg Message
}

type Dispatcher struct {
	store    *store.Store
	vault    *vault.Vault
	adapters map[string]Adapter
	q        chan job
	log      *slog.Logger

	cacheMu  sync.Mutex
	cache    []store.Channel
	cachedAt time.Time
}

var sevRank = map[string]int{"info": 0, "warning": 1, "critical": 2}

func NewDispatcher(st *store.Store, v *vault.Vault, email EmailSettings, log *slog.Logger) *Dispatcher {
	hc := &http.Client{Timeout: 10 * time.Second}
	return &Dispatcher{
		store: st, vault: v, log: log,
		q: make(chan job, 2048),
		adapters: map[string]Adapter{
			"teams":    &Teams{HTTP: hc},
			"zoom":     &Zoom{HTTP: hc},
			"whatsapp": &WhatsApp{HTTP: hc},
			"email":    &Email{Settings: email},
		},
	}
}

// Start launches n delivery workers. Notification I/O never blocks detection engines.
func (d *Dispatcher) Start(ctx context.Context, n int) {
	for i := 0; i < n; i++ {
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case j := <-d.q:
					d.deliver(ctx, j)
				}
			}
		}()
	}
}

// Enqueue is non-blocking. If the queue is saturated the event is logged and dropped
// (the incident itself is already durable in the Core DB and visible in the UI).
func (d *Dispatcher) Enqueue(ctx context.Context, m Message) {
	chans, err := d.channels(ctx)
	if err != nil {
		d.log.Error("load channels", "err", err)
		return
	}
	for _, c := range chans {
		if sevRank[m.Severity] < sevRank[c.MinSeverity] {
			continue
		}
		cfg, err := d.vault.Decrypt(c.ConfigEnc)
		if err != nil {
			d.log.Error("decrypt channel config", "channel", c.ID, "err", err)
			continue
		}
		select {
		case d.q <- job{ch: c, cfg: cfg, msg: m}:
		default:
			d.log.Warn("notification queue full, dropping", "incident", m.IncidentID, "channel", c.ID)
		}
	}
}

// SendTest pushes a synthetic message synchronously to one channel.
func (d *Dispatcher) SendTest(ctx context.Context, c store.Channel) error {
	cfg, err := d.vault.Decrypt(c.ConfigEnc)
	if err != nil {
		return err
	}
	a, ok := d.adapters[c.Kind]
	if !ok {
		return fmt.Errorf("no adapter for %s", c.Kind)
	}
	return a.Send(ctx, cfg, Message{
		Event: "opened", IncidentID: "test", Title: "Sentinel test notification", Kind: "business_exception",
		Source: "sentinel", Severity: "info", Summary: "If you can read this, the channel is wired correctly.",
		Timestamp: time.Now().UTC(), Link: "https://platform.local",
	})
}

func (d *Dispatcher) InvalidateCache() {
	d.cacheMu.Lock()
	d.cachedAt = time.Time{}
	d.cacheMu.Unlock()
}

func (d *Dispatcher) channels(ctx context.Context) ([]store.Channel, error) {
	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()
	if time.Since(d.cachedAt) < 30*time.Second {
		return d.cache, nil
	}
	c, err := d.store.ListChannels(ctx, true)
	if err != nil {
		return nil, err
	}
	d.cache, d.cachedAt = c, time.Now()
	return c, nil
}

func (d *Dispatcher) deliver(ctx context.Context, j job) {
	a, ok := d.adapters[j.ch.Kind]
	if !ok {
		return
	}
	var err error
	attempts := 0
	backoff := time.Second
	for attempts < 4 {
		attempts++
		sctx, cancel := context.WithTimeout(ctx, 15*time.Second)
		err = a.Send(sctx, j.cfg, j.msg)
		cancel()
		if err == nil {
			break
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff *= 2
	}
	status, msg := "sent", ""
	if err != nil {
		status, msg = "failed", err.Error()
		d.log.Warn("notification failed", "channel", j.ch.Name, "incident", j.msg.IncidentID, "err", err)
	}
	if j.msg.IncidentID != "test" {
		d.store.LogNotification(context.Background(), j.msg.IncidentID, j.ch.ID, j.msg.Event, status, attempts, msg)
	}
}
