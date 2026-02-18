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
)

const (
	exitOk = iota
	exitErr
)

func parseDuration(s string, defaultVal time.Duration) time.Duration {
	if s == "" {
		return defaultVal
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		slog.Warn("invalid duration, using default", "value", s, "default", defaultVal)
		return defaultVal
	}
	return d
}

func run() int {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	packageListPath := os.Getenv("PACKAGE_LIST")
	if packageListPath == "" {
		slog.Error("env var PACKAGE_LIST is required")
		return exitErr
	}

	cacheTTL := parseDuration(os.Getenv("CACHE_TTL"), 5*time.Minute)
	cacheCleanupInterval := parseDuration(os.Getenv("CACHE_CLEANUP_INTERVAL"), 10*time.Minute)
	httpClientTimeout := parseDuration(os.Getenv("HTTP_CLIENT_TIMEOUT"), 30*time.Second)
	handlerTimeout := parseDuration(os.Getenv("HANDLER_TIMEOUT"), 60*time.Second)
	gracefulDegradation := os.Getenv("GRACEFUL_DEGRADATION") == "true"

	sourceIdentifier := os.Getenv("SOURCE_IDENTIFIER")
	if sourceIdentifier == "" {
		sourceIdentifier = "api.winget-src"
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	repository, err := NewWingetSrcRepository(ctx, packageListPath, cacheTTL, cacheCleanupInterval, httpClientTimeout, gracefulDegradation)
	if err != nil {
		slog.Error(err.Error())
		return exitErr
	}
	service := NewWingetSrcService(repository, sourceIdentifier)
	handler := NewWingetSrcHandler(service, handlerTimeout)

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 30 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		slog.Info("start server listen")

		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error(err.Error())
		}
	}()

	<-ctx.Done()

	slog.Info("start server shutdown")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error(err.Error())
		return exitErr
	}

	slog.Info("done server shutdown")

	return exitOk
}

func main() {
	os.Exit(run())
}
