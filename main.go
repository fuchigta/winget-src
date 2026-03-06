package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/peterbourgon/ff/v3"
)

const (
	exitOk = iota
	exitErr
)

func run() int {
	fs := flag.NewFlagSet("winget-src", flag.ContinueOnError)

	port := fs.String("port", "8080", "listen port")
	packageListPath := fs.String("package-list", "", "path to packages.yaml (required)")
	cacheTTLStr := fs.String("cache-ttl", "5m", "cache TTL")
	cacheCleanupStr := fs.String("cache-cleanup-interval", "10m", "cache cleanup interval")
	handlerTimeoutStr := fs.String("handler-timeout", "60s", "handler timeout")
	gracefulDegradation := fs.Bool("graceful-degradation", false, "return partial results when some providers fail")
	sourceIdentifier := fs.String("source-identifier", "api.winget-src", "WinGet source identifier")
	tlsCert := fs.String("tls-cert", "", "TLS certificate file")
	tlsKey := fs.String("tls-key", "", "TLS key file")
	logLevel := fs.String("log-level", "info", "log level (debug, info, warn, error)")
	cacheMaxEntriesStr := fs.String("cache-max-entries", "0", "max number of cache entries (0 = unlimited)")
	sha256CacheFile := fs.String("sha256-cache-file", "", "path to SHA256 disk cache file (env: SHA256_CACHE_FILE)")

	if err := ff.Parse(fs, os.Args[1:], ff.WithEnvVarPrefix("")); err != nil {
		slog.Error(err.Error())
		return exitErr
	}

	var level slog.Level
	switch strings.ToLower(*logLevel) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	})))

	if *packageListPath == "" {
		slog.Error("flag -package-list (env PACKAGE_LIST) is required")
		return exitErr
	}

	if *sha256CacheFile == "" {
		*sha256CacheFile = filepath.Join(filepath.Dir(*packageListPath), "sha256_cache.json")
	}

	parseDuration := func(s string, def time.Duration) time.Duration {
		d, err := time.ParseDuration(s)
		if err != nil {
			slog.Warn("invalid duration, using default", "value", s, "default", def)
			return def
		}
		return d
	}

	cacheTTL := parseDuration(*cacheTTLStr, 5*time.Minute)
	cacheCleanupInterval := parseDuration(*cacheCleanupStr, 10*time.Minute)
	handlerTimeout := parseDuration(*handlerTimeoutStr, 60*time.Second)

	cacheMaxEntries := 0
	if n, err := strconv.Atoi(*cacheMaxEntriesStr); err == nil && n >= 0 {
		cacheMaxEntries = n
	} else if *cacheMaxEntriesStr != "0" {
		slog.Warn("invalid cache-max-entries, using 0 (unlimited)", "value", *cacheMaxEntriesStr)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	repository, err := NewWingetSrcRepository(ctx, *packageListPath, cacheTTL, cacheCleanupInterval, *gracefulDegradation, cacheMaxEntries, *sha256CacheFile)
	if err != nil {
		slog.Error(err.Error())
		return exitErr
	}
	service := NewWingetSrcService(repository, *sourceIdentifier)
	handler := NewWingetSrcHandler(service, handlerTimeout)

	srv := &http.Server{
		Addr:              ":" + *port,
		Handler:           handler,
		ReadHeaderTimeout: handlerTimeout / 2,
		ReadTimeout:       handlerTimeout,
		WriteTimeout:      handlerTimeout + 10*time.Second,
		IdleTimeout:       handlerTimeout * 2,
	}

	if *tlsCert != "" && *tlsKey != "" {
		cert, err := tls.LoadX509KeyPair(*tlsCert, *tlsKey)
		if err != nil {
			slog.Error("failed to load TLS certificate", "error", err)
			return exitErr
		}
		srv.TLSConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}
	}

	go func() {
		slog.Info("start server listen")

		var err error
		if srv.TLSConfig != nil {
			slog.Info("TLS enabled", "cert", *tlsCert, "key", *tlsKey)
			err = srv.ListenAndServeTLS("", "")
		} else {
			err = srv.ListenAndServe()
		}
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
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
