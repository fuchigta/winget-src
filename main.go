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

func run() int {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	pacakgeListPath := os.Getenv("PACKAGE_LIST")
	if pacakgeListPath == "" {
		slog.Error("env var PACKAGE_LIST is required")
		return exitErr
	}

	repository, err := NewWingetSrcRepository(pacakgeListPath)
	if err != nil {
		slog.Error(err.Error())
		return exitErr
	}
	service := NewWingetSrcService(repository)
	handler := NewWingetSrcHandler(service)

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 30 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)

	defer stop()

	go func() {
		slog.Info("start server listen")

		if err := srv.ListenAndServe(); err != nil && errors.Is(err, http.ErrServerClosed) {
			slog.Info(err.Error())
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
