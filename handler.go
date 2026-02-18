package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("failed to encode JSON response", "error", err)
	}
}

func NewWingetSrcHandler(service WingetSrcService) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	r.Get("/information", func(w http.ResponseWriter, r *http.Request) {
		res, err := service.Information(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{
				{
					ErrorCode:    http.StatusInternalServerError,
					ErrorMessage: err.Error(),
				},
			})
			return
		}
		writeJSON(w, http.StatusOK, DataResponse{Data: res})
	})

	r.Post("/manifestSearch", func(w http.ResponseWriter, r *http.Request) {
		var req ManifestSearchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, ErrorResponse{
				{
					ErrorCode:    http.StatusBadRequest,
					ErrorMessage: err.Error(),
				},
			})
			return
		}

		res, err := service.ManifestSearch(r.Context(), req)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{
				{
					ErrorCode:    http.StatusInternalServerError,
					ErrorMessage: err.Error(),
				},
			})
			return
		}

		writeJSON(w, http.StatusOK, DataResponse{Data: res})
	})

	r.Get("/packageManifests/{identifier}", func(w http.ResponseWriter, r *http.Request) {
		identifier := chi.URLParam(r, "identifier")
		version := r.URL.Query().Get("Version")

		res, err := service.PackageManifests(r.Context(), identifier, version)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{
				{
					ErrorCode:    http.StatusInternalServerError,
					ErrorMessage: err.Error(),
				},
			})
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
