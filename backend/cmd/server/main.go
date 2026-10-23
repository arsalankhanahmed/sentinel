// Sentinel — Zero-Trust Operational Exception & Observability Platform.
// Single static binary: API + scheduler + watchers + probes + notification dispatcher.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"sentinel/internal/api"
	"sentinel/internal/config"
	"sentinel/internal/connectors"
	"sentinel/internal/engine"
	"sentinel/internal/incident"
	"sentinel/internal/notify"
	"sentinel/internal/store"
	"sentinel/internal/vault"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if err := run(log); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg := config.Load()
	if !cfg.AuthDisabled && cfg.JWTSecret == "" {
		return errors.New("JWT_SECRET is required (or AUTH_DISABLED=true for local dev)")
	}
	if cfg.AuthDisabled {
		log.Warn("AUTH_DISABLED=true — never run like this in production")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	v, err := vault.New(cfg.MasterKey)
	if err != nil {
		return err
	}
	st, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer st.Close()
	if err := st.Migrate(ctx); err != nil {
		return err
	}

	disp := notify.NewDispatcher(st, v, notify.EmailSettings{
		Host: cfg.SMTPHost, Port: cfg.SMTPPort, User: cfg.SMTPUser, Pass: cfg.SMTPPass, From: cfg.SMTPFrom,
	}, log)
	disp.Start(ctx, 8)

	conns := connectors.NewPool(v)
	inc := incident.NewService(st, disp, cfg.PublicBaseURL, cfg.LinkTTL, log)
	eng := engine.New(cfg, st, conns, inc, log)
	if err := eng.Start(ctx); err != nil {
		return err
	}

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           api.NewServer(cfg, st, v, conns, eng, disp, log).Router(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	errCh := make(chan error, 1)
	go func() {
		log.Info("http listening", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
	case err := <-errCh:
		return err
	}
	log.Info("shutting down")
	shCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	_ = srv.Shutdown(shCtx)
	eng.Stop()
	return nil
}
