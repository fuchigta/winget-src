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
	handlerTimeoutStr := fs.String("handler-timeout", "60s", "handler timeout")
	gracefulDegradation := fs.Bool("graceful-degradation", false, "return partial results when some providers fail")
	sourceIdentifier := fs.String("source-identifier", "api.winget-src", "WinGet source identifier")
	tlsCert := fs.String("tls-cert", "", "TLS certificate file")
	tlsKey := fs.String("tls-key", "", "TLS key file")
	logLevel := fs.String("log-level", "info", "log level (debug, info, warn, error)")
	logFormat := fs.String("log-format", "logfmt", "log format (logfmt, json)")
	refreshIntervalStr := fs.String("refresh-interval", "5m", "interval for automatic cache refresh (0 to disable)")
	versionCacheFile := fs.String("version-cache-file", "", "path to version disk cache file (default: same dir as package-list)")
	check := fs.Bool("check", false, "check if cache can be built for all packages without starting the server")

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
	handlerOpts := &slog.HandlerOptions{Level: level}
	var logHandler slog.Handler
	if strings.ToLower(*logFormat) == "json" {
		logHandler = slog.NewJSONHandler(os.Stderr, handlerOpts)
	} else {
		logHandler = slog.NewTextHandler(os.Stderr, handlerOpts)
	}
	slog.SetDefault(slog.New(logHandler))

	if *packageListPath == "" {
		slog.Error("flag -package-list (env PACKAGE_LIST) is required")
		return exitErr
	}

	if *versionCacheFile == "" {
		*versionCacheFile = filepath.Join(filepath.Dir(*packageListPath), "version_cache.json")
	}

	parseDuration := func(s string, def time.Duration) time.Duration {
		d, err := time.ParseDuration(s)
		if err != nil {
			slog.Warn("invalid duration, using default", "value", s, "default", def)
			return def
		}
		return d
	}

	refreshInterval := parseDuration(*refreshIntervalStr, 5*time.Minute)
	handlerTimeout := parseDuration(*handlerTimeoutStr, 60*time.Second)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if *check {
		return runCheck(ctx, *packageListPath)
	}

	repository, err := NewWingetSrcRepository(ctx, *packageListPath, refreshInterval, *gracefulDegradation, *versionCacheFile)
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
