// Package incident turns detections into durable, de-duplicated incidents with
// secure deep links, and hands compliance-safe messages to the dispatcher.
package incident

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"log/slog"
	"time"

	"sentinel/internal/notify"
	"sentinel/internal/store"
)

type Service struct {
	store   *store.Store
	disp    *notify.Dispatcher
	baseURL string
	ttl     time.Duration
	log     *slog.Logger
}

func NewService(st *store.Store, d *notify.Dispatcher, baseURL string, ttl time.Duration, log *slog.Logger) *Service {
	return &Service{store: st, disp: d, baseURL: baseURL, ttl: ttl, log: log}
}

// NewLinkToken returns (plaintext token for the URL, sha256 hash for storage).
// 256 bits of entropy: unguessable, and a DB leak does not leak usable links.
func NewLinkToken() (string, []byte) {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	tok := base64.RawURLEncoding.EncodeToString(b)
	return tok, HashToken(tok)
}

func HashToken(tok string) []byte {
	h := sha256.Sum256([]byte(tok))
	return h[:]
}

// Raise opens (or refreshes) an incident. Notifications fire only for NEW incidents,
// which prevents alert storms while a condition persists.
func (s *Service) Raise(ctx context.Context, n store.NewIncident) {
	tok, hash := NewLinkToken()
	n.LinkTokenHash = hash
	n.LinkExpiresAt = time.Now().Add(s.ttl)
	if n.CorrectiveAction == "" {
		n.CorrectiveAction = Playbook(n.Kind, n.Summary)
	}
	id, inserted, err := s.store.UpsertIncident(ctx, n)
	if err != nil {
		s.log.Error("upsert incident", "fingerprint", n.Fingerprint, "err", err)
		return
	}
	if !inserted {
		return
	}
	s.log.Info("incident opened", "id", id, "kind", n.Kind, "source", n.SourceName, "rows", len(n.Rows))
	s.disp.Enqueue(ctx, notify.Message{
		Event: "opened", IncidentID: id, Title: n.Title, Kind: n.Kind, Source: n.SourceName,
		Severity: n.Severity, RowCount: len(n.Rows), Summary: n.Summary, Timestamp: time.Now().UTC(),
		Link: s.baseURL + "/i/" + tok,
	})
}

// Resolve auto-closes the active incident for a fingerprint (condition cleared).
func (s *Service) Resolve(ctx context.Context, fingerprint string) {
	inc, err := s.store.ResolveActive(ctx, fingerprint)
	if err != nil {
		s.log.Error("resolve incident", "fingerprint", fingerprint, "err", err)
		return
	}
	if inc == nil {
		return
	}
	s.log.Info("incident auto-resolved", "id", inc.ID, "source", inc.SourceName)
	s.disp.Enqueue(ctx, notify.Message{
		Event: "resolved", IncidentID: inc.ID, Title: inc.Title, Kind: inc.Kind, Source: inc.SourceName,
		Severity: inc.Severity, Summary: "Condition cleared — incident auto-resolved.", Timestamp: time.Now().UTC(),
	})
}
