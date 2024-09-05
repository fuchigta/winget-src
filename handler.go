package main

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func NewWingetSrcHandler(service WingetSrcService) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	r.Get("/information", func(w http.ResponseWriter, r *http.Request) {
		res, _ := service.Information()

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(DataResponse{
			Data: res,
		})
	})

	r.Post("/manifestSearch", func(w http.ResponseWriter, r *http.Request) {
		var req ManifestSearchRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.Header().Add("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(ErrorResponse{
				{
					ErrorCode:    http.StatusBadRequest,
					ErrorMessage: err.Error(),
				},
			})
			return
		}

		res, err := service.ManifestSearch(req)
		if err != nil {
			w.Header().Add("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(ErrorResponse{
				{
					ErrorCode:    http.StatusInternalServerError,
					ErrorMessage: err.Error(),
				},
			})
			return
		}

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(DataResponse{
			Data: res,
		})
	})

	r.Get("/packageManifests/{identifier}", func(w http.ResponseWriter, r *http.Request) {
		identifier := chi.URLParam(r, "identifier")
		version := r.URL.Query().Get("Version")

		res, err := service.PackageManifests(identifier, version)
		if err != nil {
			w.Header().Add("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(ErrorResponse{
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

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(DataResponse{
			Data: res,
		})
	})

	return r
}
