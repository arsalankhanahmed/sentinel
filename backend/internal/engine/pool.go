package engine

import (
	"context"
	"log/slog"
	"runtime/debug"
	"sync"
)

// WorkerPool is a bounded, keyed worker pool:
//   - fixed number of goroutines ⇒ predictable load on monitored databases
//   - non-blocking Submit ⇒ schedulers / tickers are never blocked
//   - per-key in-flight guard ⇒ a slow job is never run twice concurrently
type WorkerPool struct {
	jobs     chan task
	inflight sync.Map
	log      *slog.Logger
	wg       sync.WaitGroup
}

type task struct {
	key string
	fn  func(ctx context.Context)
}

func NewWorkerPool(size, queue int, log *slog.Logger) *WorkerPool {
	return &WorkerPool{jobs: make(chan task, queue), log: log, wg: sync.WaitGroup{}}
}

func (p *WorkerPool) Start(ctx context.Context, size int) {
	for i := 0; i < size; i++ {
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case t := <-p.jobs:
					p.run(ctx, t)
				}
			}
		}()
	}
}

func (p *WorkerPool) run(ctx context.Context, t task) {
	defer p.inflight.Delete(t.key)
	defer func() {
		if r := recover(); r != nil {
			p.log.Error("job panic", "key", t.key, "panic", r, "stack", string(debug.Stack()))
		}
	}()
	t.fn(ctx)
}

// Submit returns false if the same key is already queued/running or the queue is full.
func (p *WorkerPool) Submit(key string, fn func(ctx context.Context)) bool {
	if _, busy := p.inflight.LoadOrStore(key, struct{}{}); busy {
		return false
	}
	select {
	case p.jobs <- task{key: key, fn: fn}:
		return true
	default:
		p.inflight.Delete(key)
		p.log.Warn("worker queue full, skipping", "key", key)
		return false
	}
}

func (p *WorkerPool) Wait() { p.wg.Wait() }
