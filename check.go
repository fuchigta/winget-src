package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

func runCheck(ctx context.Context, packageListPath, versionCacheFile string) int {
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

	versionCache := NewVersionCache(versionCacheFile)
	httpClient := &http.Client{}
	sha256Fetcher := NewSHA256Fetcher(httpClient, 10*time.Minute)

	urlToSHA256 := make(map[string]string)
	for _, versions := range versionCache.All() {
		for _, v := range versions {
			for _, inst := range v.Installers {
				if inst.InstallerSha256 != "" {
					urlToSHA256[inst.InstallerUrl] = inst.InstallerSha256
				}
			}
		}
	}
	sha256Fetcher.Seed(urlToSHA256)

	okCount := 0
	failCount := 0

	for _, entry := range packageList {
		provider, err := dispatchProvider(entry, httpClient, sha256Fetcher)
		if err != nil {
			slog.Error("check: failed", "package", entry.Id, "error", err)
			failCount++
			continue
		}

		versions, err := fetchWithRetry(ctx, entry.Id, entry.Provider, "fetch versions", func() ([]Version, error) {
			return provider.FetchVersions(ctx, entry)
		})
		if err != nil {
			slog.Error("check: failed", "package", entry.Id, "error", err)
			failCount++
			continue
		}

		slog.Info("check: ok", "package", entry.Id, "versions", len(versions))
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
