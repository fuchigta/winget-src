package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"

	"gopkg.in/yaml.v3"
)

func runCheck(ctx context.Context, packageListPath string, allowNoReleases bool) int {
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

			adapter, ok := provider.(releaseAdapter)
			if !ok {
				results[i] = result{pkg: entry.Id, err: fmt.Errorf("provider %T does not support release check", provider)}
				return
			}

			cfg, err := getInstallerConfig(entry.InstallerType)
			if err != nil {
				results[i] = result{pkg: entry.Id, err: err}
				return
			}

			releases, err := fetchWithRetry(ctx, entry.Id, entry.Provider, "fetch releases", func() ([]release, error) {
				return fetchReleases(ctx, httpClient, adapter, entry)
			})
			if err != nil {
				results[i] = result{pkg: entry.Id, err: err}
				return
			}

			if len(releases) == 0 {
				if allowNoReleases {
					slog.Warn("check: no releases found (skipped)", "package", entry.Id)
					results[i] = result{pkg: entry.Id}
				} else {
					results[i] = result{pkg: entry.Id, err: fmt.Errorf("no releases found")}
				}
				return
			}

			matched := countMatchingReleases(releases, cfg)
			if matched == 0 {
				results[i] = result{pkg: entry.Id, err: fmt.Errorf("releases found (%d) but no versions could be built for installer type %q", len(releases), entry.InstallerType)}
				return
			}

			slog.Info("check: ok", "package", entry.Id, "releases", len(releases), "matched", matched)
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
