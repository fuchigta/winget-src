package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"gopkg.in/yaml.v3"
)

func runCheck(ctx context.Context, packageListPath string) int {
	f, err := os.Open(packageListPath)
	if err != nil {
		slog.Error("check: failed to open package list", "error", err)
		return exitErr
	}
	defer f.Close()

	var packageList []PackageListEntry
	if err := yaml.NewDecoder(f).Decode(&packageList); err != nil {
		slog.Error("check: failed to parse package list", "error", err)
		return exitErr
	}

	httpClient := &http.Client{}

	okCount := 0
	failCount := 0

	for _, entry := range packageList {
		provider, err := dispatchProvider(entry, httpClient, nil)
		if err != nil {
			slog.Error("check: failed", "package", entry.Id, "error", err)
			failCount++
			continue
		}

		names, err := fetchWithRetry(ctx, entry.Id, entry.Provider, "fetch release names", func() ([]string, error) {
			return provider.FetchReleaseNames(ctx, entry)
		})
		if err != nil {
			slog.Error("check: failed", "package", entry.Id, "error", err)
			failCount++
			continue
		}

		slog.Info("check: ok", "package", entry.Id, "releases", len(names))
		okCount++
	}

	total := okCount + failCount
	if failCount > 0 {
		slog.Error("check: completed with failures", "ok", okCount, "failed", failCount, "total", total)
		return exitErr
	}

	slog.Info("check: completed successfully", "ok", okCount, "total", total)
	return exitOk
}
