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

// checkPackage validates that the given entry can produce at least one buildable version.
// Returns nil on success, or an error describing the validation failure.
func checkPackage(ctx context.Context, httpClient *http.Client, entry PackageListEntry, allowNoReleases bool) error {
	provider, err := dispatchProvider(entry, httpClient, nil)
	if err != nil {
		return err
	}

	adapter, ok := provider.(releaseAdapter)
	if !ok {
		return fmt.Errorf("provider %T does not support release check", provider)
	}

	cfg, err := getInstallerConfig(entry.InstallerType)
	if err != nil {
		return err
	}

	releases, err := fetchWithRetry(ctx, entry.Id, entry.Provider, "fetch releases", func() ([]release, error) {
		return fetchReleases(ctx, httpClient, adapter, entry)
	})
	if err != nil {
		return err
	}

	if len(releases) == 0 {
		if allowNoReleases {
			slog.Warn("check: no releases found (skipped)", "package", entry.Id)
			return nil
		}
		return fmt.Errorf("no releases found")
	}

	matched := countMatchingReleases(releases, cfg)
	if matched == 0 {
		return fmt.Errorf("releases found (%d) but no versions could be built for installer type %q", len(releases), entry.InstallerType)
	}

	slog.Info("check: ok", "package", entry.Id, "releases", len(releases), "matched", matched)
	return nil
}

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

	for _, entry := range packageList {
		if err := validateEntry(entry); err != nil {
			slog.Error("check: invalid config", "package", entry.Id, "error", err)
			return exitErr
		}
		if err := validateArchitecture(entry.Architecture); err != nil {
			slog.Error("check: invalid config", "package", entry.Id, "error", err)
			return exitErr
		}
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
			err := checkPackage(ctx, httpClient, entry, allowNoReleases)
			results[i] = result{pkg: entry.Id, err: err}
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
