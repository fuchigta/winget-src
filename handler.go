package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type slogLogFormatter struct{}

func (f *slogLogFormatter) NewLogEntry(r *http.Request) middleware.LogEntry {
	return &slogLogEntry{r: r}
}

type slogLogEntry struct {
	r *http.Request
}

func (e *slogLogEntry) Write(status, bytes int, _ http.Header, elapsed time.Duration, _ any) {
	slog.Info("access",
		"method", e.r.Method,
		"path", e.r.RequestURI,
		"status", status,
		"bytes", bytes,
		"elapsed", elapsed,
	)
}

func (e *slogLogEntry) Panic(v any, stack []byte) {
	slog.Error("panic", "error", v, "stack", string(stack))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("failed to encode JSON response", "error", err)
	}
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, ErrorResponse{{ErrorCode: status, ErrorMessage: err.Error()}})
}

func NewWingetSrcHandler(service WingetSrcService, timeout time.Duration) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.RequestLogger(&slogLogFormatter{}))
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(timeout))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	r.Get("/information", func(w http.ResponseWriter, r *http.Request) {
		res, err := service.Information(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, DataResponse{Data: res})
	})

	r.Post("/manifestSearch", func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxResponseBodySize)
		var req ManifestSearchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}

		res, err := service.ManifestSearch(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}

		writeJSON(w, http.StatusOK, DataResponse{Data: res})
	})

	r.Get("/packageManifests/*", func(w http.ResponseWriter, r *http.Request) {
		identifier := chi.URLParam(r, "*")
		version := r.URL.Query().Get("Version")

		res, err := service.PackageManifests(r.Context(), identifier, version)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}

		if res.PackageIdentifier == "" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		writeJSON(w, http.StatusOK, DataResponse{Data: res})
	})

	return r
}
