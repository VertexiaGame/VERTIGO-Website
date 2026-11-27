package main

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"vertexia-frontend/backend/gsmagent"
	"vertexia-frontend/backend/service/gsm"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, gsmagent.ErrorResponse{Error: msg})
}

// routes returns the authenticated HTTP API. Every route, health included,
// sits behind the HMAC middleware.
func (a *Agent) routes(nonces gsmagent.NonceStore) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST "+gsmagent.PathServers, a.handleCreate)
	mux.HandleFunc("GET "+gsmagent.PathServers, a.handleList)
	mux.HandleFunc("GET "+gsmagent.PathServers+"/{id}", a.handleGet)
	mux.HandleFunc("DELETE "+gsmagent.PathServers+"/{id}", a.handleStop)
	mux.HandleFunc("GET "+gsmagent.PathHealth, a.handleHealth)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not found")
	})
	authed := gsmagent.Middleware([]byte(a.cfg.Secret), nonces, a.log, mux)
	return a.accessLog(recoverer(a.log, authed))
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (a *Agent) accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		a.log.Info("request", "method", r.Method, "path", r.URL.Path, "status", rec.status, "remote", r.RemoteAddr, "duration_ms", time.Since(start).Milliseconds())
	})
}

func recoverer(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				log.Error("handler panic", "path", r.URL.Path, "panic", v)
				writeError(w, http.StatusInternalServerError, "internal error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (a *Agent) handleCreate(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, gsmagent.MaxRequestBody))
	if err != nil {
		writeError(w, http.StatusBadRequest, "unreadable body")
		return
	}
	req, err := decodeCreate(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	// Readiness is awaited even if the caller disconnects so the record and
	// port are always settled; the platform stops servers it gave up on.
	srv, err := a.Create(req)
	if err == nil {
		writeJSON(w, http.StatusCreated, srv)
		return
	}
	if errors.Is(err, errCapacity) {
		writeError(w, http.StatusConflict, gsmagent.ErrorCapacity)
		return
	}
	var ce *createError
	if errors.As(err, &ce) {
		if ce.status == http.StatusConflict {
			writeJSON(w, http.StatusConflict, srv)
			return
		}
		writeError(w, ce.status, ce.msg)
		return
	}
	writeError(w, http.StatusInternalServerError, "internal error")
}

func (a *Agent) handleList(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, gsmagent.ListServersResponse{Servers: a.List()})
}

func (a *Agent) handleGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !gsm.ValidID(id) {
		writeError(w, http.StatusBadRequest, "invalid server id")
		return
	}
	srv, ok := a.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "server not found")
		return
	}
	writeJSON(w, http.StatusOK, srv)
}

func (a *Agent) handleStop(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !gsm.ValidID(id) {
		writeError(w, http.StatusBadRequest, "invalid server id")
		return
	}
	if err := a.Stop(id); err != nil {
		if errors.Is(err, errNotFound) {
			writeError(w, http.StatusNotFound, "server not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, gsmagent.StopServerResponse{Stopped: true})
}

func (a *Agent) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, a.Health())
}
