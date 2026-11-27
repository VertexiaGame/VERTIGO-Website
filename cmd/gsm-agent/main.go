// Command gsm-agent runs on a dedicated Linux gameserver host. It launches,
// supervises and stops RaveEngineServer processes on behalf of the platform,
// which drives it over an HMAC-signed HTTPS API (see backend/gsmagent).
//
// The agent never kills gameservers when it exits: on SIGTERM/SIGINT it stops
// serving HTTP and returns, leaving the processes running (systemd
// KillMode=process). The next agent run reconciles from Redis and adopts them.
package main

import (
	"context"
	"crypto/tls"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"vertexia-frontend/backend/gsmagent"
	"vertexia-frontend/backend/kv"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)
	if err := run(logger); err != nil {
		logger.Error("gsm-agent exiting", "error", err.Error())
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := loadConfig(os.Getenv)
	if err != nil {
		return err
	}
	if st, err := os.Stat(cfg.ServerBin); err != nil || !st.Mode().IsRegular() {
		return errors.New("GSM_SERVER_BIN does not point to a regular file")
	}
	if err := os.MkdirAll(cfg.LogsDir, 0750); err != nil {
		return err
	}
	logger = logger.With("agent_id", cfg.AgentID)

	rc, err := kv.Connect(cfg.Redis)
	if err != nil {
		return errors.New("redis connection failed: " + err.Error())
	}
	defer rc.Close()

	agent := newAgent(cfg, newRedisRecordStore(rc, cfg.AgentID), logger)
	agent.ports.probe = udpProbe(cfg.BindHost)
	reconcileCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	err = agent.Reconcile(reconcileCtx)
	cancel()
	if err != nil {
		return errors.New("reconcile failed: " + err.Error())
	}

	srv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           agent.routes(gsmagent.NewRedisNonceStore(rc)),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      cfg.ReadyTimeout + 2*cfg.ShutdownGrace + 30*time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    16 << 10,
		TLSConfig:         &tls.Config{MinVersion: tls.VersionTLS12},
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelWarn),
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		tlsMode := cfg.TLSCert != ""
		logger.Info("gsm-agent listening", "addr", cfg.Listen, "tls", tlsMode, "ports", cfg.PortMin, "ports_max", cfg.PortMax, "version", version)
		var err error
		if tlsMode {
			err = srv.ListenAndServeTLS(cfg.TLSCert, cfg.TLSKey)
		} else {
			err = srv.ListenAndServe()
		}
		if !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received; gameservers stay running")
	case err := <-errCh:
		if err != nil {
			agent.Close(5 * time.Second)
			return err
		}
	}
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelShutdown()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Warn("http shutdown incomplete", "error", err.Error())
	}
	agent.Close(5 * time.Second)
	logger.Info("gsm-agent stopped")
	return nil
}
