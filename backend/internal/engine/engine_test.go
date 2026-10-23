package engine

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

func TestValidateSchedule(t *testing.T) {
	for _, s := range []string{"*/5 * * * *", "0 8 * * 1", "30 0 8 * * *", "@every 30s", "@hourly", "@daily"} {
		if err := ValidateSchedule(s); err != nil {
			t.Errorf("%q should be valid: %v", s, err)
		}
	}
	for _, s := range []string{"", "every banana", "61 * * * *", "* * *"} {
		if ValidateSchedule(s) == nil {
			t.Errorf("%q should be invalid", s)
		}
	}
}

func TestPoolRunsJobs(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p := NewWorkerPool(4, 100, quiet)
	p.Start(ctx, 4)
	var n atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		key := string(rune('a'+i%26)) + string(rune('A'+i/26))
		if !p.Submit(key, func(context.Context) { n.Add(1); wg.Done() }) {
			wg.Done()
		}
	}
	wg.Wait()
	if n.Load() != 50 {
		t.Fatalf("ran %d jobs", n.Load())
	}
}

// A slow job must never run twice concurrently (e.g. a long rule overlapping its next cron tick).
func TestPoolDeduplicatesInflightKey(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p := NewWorkerPool(2, 10, quiet)
	p.Start(ctx, 2)
	release := make(chan struct{})
	started := make(chan struct{})
	if !p.Submit("rule:1", func(context.Context) { close(started); <-release }) {
		t.Fatal("first submit rejected")
	}
	<-started
	if p.Submit("rule:1", func(context.Context) {}) {
		t.Fatal("duplicate in-flight key must be rejected")
	}
	if !p.Submit("rule:2", func(context.Context) {}) {
		t.Fatal("different key must be accepted")
	}
	close(release)
	deadline := time.Now().Add(3 * time.Second)
	for !p.Submit("rule:1", func(context.Context) {}) {
		if time.Now().After(deadline) {
			t.Fatal("key must be reusable after completion")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestPoolSurvivesPanic(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p := NewWorkerPool(1, 10, quiet)
	p.Start(ctx, 1)
	p.Submit("boom", func(context.Context) { panic("kaboom") })
	done := make(chan struct{})
	p.Submit("after", func(context.Context) { close(done) })
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("worker died after panic")
	}
}

func TestPoolNonBlockingWhenFull(t *testing.T) {
	p := NewWorkerPool(1, 1, quiet) // not started: queue fills immediately
	if !p.Submit("a", func(context.Context) {}) {
		t.Fatal("first should queue")
	}
	start := time.Now()
	if p.Submit("b", func(context.Context) {}) {
		t.Fatal("second should be rejected when queue is full")
	}
	if time.Since(start) > 500*time.Millisecond { // generous for slow CI runners with -race
		t.Fatal("Submit must never block the scheduler")
	}
}

func TestLookupToleratesQuotedColumns(t *testing.T) {
	row := map[string]any{"LogId": int64(9)}
	for _, c := range []string{"LogId", "[LogId]", `"LogId"`, "logid", "dbo.LogId"} {
		if lookup(row, c) != int64(9) {
			t.Errorf("lookup(%q) failed", c)
		}
	}
}
