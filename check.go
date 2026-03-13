package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"sync"

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

	type result struct {
		pkg string
		err error
	}

	results := make([]result, len(packageList))
	var wg sync.WaitGroup
	wg.Add(len(packageList))

	for i, entry := range packageList {
		i, entry := i, entry
		go func() {
			defer wg.Done()
			provider, err := dispatchProvider(entry, httpClient, nil)
			if err != nil {
				results[i] = result{pkg: entry.Id, err: err}
				return
			}

			names, err := fetchWithRetry(ctx, entry.Id, entry.Provider, "fetch release names", func() ([]string, error) {
				return provider.FetchReleaseNames(ctx, entry)
			})
			if err != nil {
				results[i] = result{pkg: entry.Id, err: err}
				return
			}

			slog.Info("check: ok", "package", entry.Id, "releases", len(names))
			results[i] = result{pkg: entry.Id}
		}()
	}

	wg.Wait()

	okCount := 0
	failCount := 0
	for _, r := range results {
		if r.err != nil {
			slog.Error("check: failed", "package", r.pkg, "error", r.err)
			failCount++
		} else {
			okCount++
		}
	}

	total := okCount + failCount
	if failCount > 0 {
		slog.Error("check: completed with failures", "ok", okCount, "failed", failCount, "total", total)
		return exitErr
	}

	slog.Info("check: completed successfully", "ok", okCount, "total", total)
	return exitOk
}
