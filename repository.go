package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

type QueryManifestCondition func(PackageListEntry) bool

type WingetSrcRepository interface {
	QueryManifest(ctx context.Context, condition QueryManifestCondition) ([]Manifest, error)
	QueryPackageManifests(ctx context.Context, identifier string, version string) (PackageManifests, error)
}

type WingetSrcRepositoryImpl struct {
	packageList         []PackageListEntry
	packageMap          map[string]PackageListEntry
	versionCache        *VersionCache
	httpClient          *http.Client
	sha256Fetcher       *SHA256Fetcher
	gracefulDegradation bool
}

func ById(id string) QueryManifestCondition {
	return func(entry PackageListEntry) bool {
		return strings.Contains(strings.ToLower(entry.Id), strings.ToLower(id))
	}
}

func ByName(name string) QueryManifestCondition {
	return func(entry PackageListEntry) bool {
		return strings.Contains(strings.ToLower(entry.Name), strings.ToLower(name))
	}
}

func Or(conditions ...QueryManifestCondition) QueryManifestCondition {
	return func(entry PackageListEntry) bool {
		for _, condition := range conditions {
			if condition(entry) {
				return true
			}
		}

		return false
	}
}

func And(conditions ...QueryManifestCondition) QueryManifestCondition {
	return func(entry PackageListEntry) bool {
		for _, condition := range conditions {
			if !condition(entry) {
				return false
			}
		}

		return true
	}
}

// fetchWithRetry calls fn up to 3 times with exponential backoff, logging each attempt.
func fetchWithRetry[T any](ctx context.Context, pkg, provider, opName string, fn func() (T, error)) (T, error) {
	const maxAttempts = 3
	backoff := time.Second

	var zero T
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		start := time.Now()
		result, err := fn()
		elapsed := time.Since(start)

		if err == nil {
			slog.Debug("provider "+opName+" succeeded",
				"package", pkg,
				"provider", provider,
				"latency_ms", elapsed.Milliseconds(),
			)
			return result, nil
		}

		slog.Warn("provider "+opName+" failed",
			"package", pkg,
			"provider", provider,
			"latency_ms", elapsed.Milliseconds(),
			"attempt", attempt,
			"error", err,
		)

		if attempt == maxAttempts {
			return zero, fmt.Errorf("%s (after %d attempts): %w", opName, maxAttempts, err)
		}

		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		case <-time.After(backoff):
		}

		if backoff < 8*time.Second {
			backoff *= 2
		}
	}
	panic("unreachable")
}

func (w WingetSrcRepositoryImpl) fetchVersionsCached(ctx context.Context, entry PackageListEntry) ([]Version, error) {
	if cached, ok := w.versionCache.Get(entry.Id); ok {
		return cached, nil
	}

	provider, err := dispatchProvider(entry, w.httpClient, w.sha256Fetcher)
	if err != nil {
		return nil, err
	}

	versions, err := fetchWithRetry(ctx, entry.Id, entry.Provider, "fetch versions", func() ([]Version, error) {
		return provider.FetchVersions(ctx, entry)
	})
	if err != nil {
		return nil, err
	}

	w.versionCache.Set(entry.Id, versions)
	return versions, nil
}

func (w WingetSrcRepositoryImpl) fetchReleaseNamesCached(ctx context.Context, entry PackageListEntry) ([]string, error) {
	if cached, ok := w.versionCache.Get(entry.Id); ok {
		names := make([]string, len(cached))
		for i, v := range cached {
			names[i] = v.Version
		}
		return names, nil
	}

	provider, err := dispatchProvider(entry, w.httpClient, w.sha256Fetcher)
	if err != nil {
		return nil, err
	}

	return fetchWithRetry(ctx, entry.Id, entry.Provider, "fetch release names", func() ([]string, error) {
		return provider.FetchReleaseNames(ctx, entry)
	})
}

// fetchFilteredVersions fetches versions and computes SHA256 only for the specified target versions.
func (w WingetSrcRepositoryImpl) fetchFilteredVersions(ctx context.Context, entry PackageListEntry, targetVersions []string) ([]Version, error) {
	provider, err := dispatchProvider(entry, w.httpClient, w.sha256Fetcher)
	if err != nil {
		return nil, err
	}
	adapter, ok := provider.(releaseAdapter)
	if !ok {
		return nil, fmt.Errorf("provider %T does not support filtered fetch", provider)
	}
	return fetchAndBuildVersions(ctx, w.httpClient, w.sha256Fetcher, adapter, entry, targetVersions)
}

func (w WingetSrcRepositoryImpl) QueryManifest(ctx context.Context, condition QueryManifestCondition) ([]Manifest, error) {
	matched := []PackageListEntry{}
	for _, entry := range w.packageList {
		if condition(entry) {
			matched = append(matched, entry)
		}
	}

	if len(matched) == 0 {
		return []Manifest{}, nil
	}

	type result struct {
		index    int
		manifest Manifest
		err      error
	}

	results := make([]result, len(matched))
	var wg sync.WaitGroup
	wg.Add(len(matched))

	for i, entry := range matched {
		i, entry := i, entry
		go func() {
			defer wg.Done()
			names, err := w.fetchReleaseNamesCached(ctx, entry)
			if err != nil {
				results[i] = result{index: i, err: err}
				return
			}
			manifestVersions := []ManifestVersion{}
			for _, name := range names {
				manifestVersions = append(manifestVersions, ManifestVersion{
					PackageVersion: name,
				})
			}
			results[i] = result{
				index: i,
				manifest: Manifest{
					PackageIdentifier: entry.PackageIdentifier(),
					PackageName:       entry.Name,
					Publisher:         entry.Publisher,
					Versions:          manifestVersions,
				},
			}
		}()
	}

	wg.Wait()

	manifests := []Manifest{}
	for _, r := range results {
		if r.err != nil {
			if w.gracefulDegradation {
				slog.Warn("failed to fetch release names, skipping package", "package", matched[r.index].Id, "error", r.err)
				continue
			}
			return nil, r.err
		}
		manifests = append(manifests, r.manifest)
	}

	return manifests, nil
}

func (w WingetSrcRepositoryImpl) QueryPackageManifests(ctx context.Context, identifier string, version string) (PackageManifests, error) {
	found, ok := w.packageMap[strings.ToLower(identifier)]
	if !ok {
		return PackageManifests{}, nil
	}

	var versions []Version
	var err error
	if version != "" {
		versions, err = w.fetchFilteredVersions(ctx, found, []string{version})
	} else {
		versions, err = w.fetchVersionsCached(ctx, found)
	}
	if err != nil {
		return PackageManifests{}, err
	}

	pkgManifestVersions := []PackageManifestsVersion{}

	for _, v := range versions {
		pkgManifestVersions = append(pkgManifestVersions, PackageManifestsVersion{
			PackageVersion: v.Version,
			Installers:     v.Installers,
			DefaultLocale: Locale{
				PackageName:      found.Name,
				PackageLocale:    found.GetLocale(),
				Publisher:        found.Publisher,
				License:          found.License,
				ShortDescription: found.Description,
			},
		})
	}

	return PackageManifests{
		PackageIdentifier: found.PackageIdentifier(),
		Versions:          pkgManifestVersions,
	}, nil
}

func dispatchProvider(entry PackageListEntry, httpClient *http.Client, sha256Fetcher *SHA256Fetcher) (PackageProvider, error) {
	switch entry.Provider {
	case "github":
		return Github{httpClient: httpClient, sha256Fetcher: sha256Fetcher}, nil
	case "gitlab":
		return Gitlab{httpClient: httpClient, sha256Fetcher: sha256Fetcher}, nil
	default:
		return nil, fmt.Errorf("unknown package provider: %s", entry.Provider)
	}
}

func (w WingetSrcRepositoryImpl) refreshAll() {
	slog.Info("cache refresh: starting", "packages", len(w.packageList))
	for _, entry := range w.packageList {
		provider, err := dispatchProvider(entry, w.httpClient, w.sha256Fetcher)
		if err != nil {
			slog.Warn("cache refresh: dispatch failed", "package", entry.Id, "error", err)
			continue
		}
		versions, err := fetchWithRetry(context.Background(), entry.Id, entry.Provider, "fetch versions", func() ([]Version, error) {
			return provider.FetchVersions(context.Background(), entry)
		})
		if err != nil {
			slog.Warn("cache refresh: failed", "package", entry.Id, "error", err)
			continue
		}
		w.versionCache.Set(entry.Id, versions)
	}
	slog.Info("cache refresh: completed")
}

func (w WingetSrcRepositoryImpl) startAutoRefresh(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				w.refreshAll()
			}
		}
	}()
}

func NewWingetSrcRepository(ctx context.Context, packageListPath string, refreshInterval time.Duration, gracefulDegradation bool, versionCacheFile string) (WingetSrcRepository, error) {
	f, err := os.Open(packageListPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var packageList []PackageListEntry
	if err := yaml.NewDecoder(f).Decode(&packageList); err != nil {
		return nil, err
	}

	for _, entry := range packageList {
		if err := validateEntry(entry); err != nil {
			return nil, fmt.Errorf("package %s: %w", entry.Id, err)
		}
		if err := validateArchitecture(entry.Architecture); err != nil {
			return nil, fmt.Errorf("package %s: %w", entry.Id, err)
		}
	}

	packageMap := make(map[string]PackageListEntry, len(packageList))
	for _, entry := range packageList {
		packageMap[strings.ToLower(entry.PackageIdentifier())] = entry
	}

	versionCache := NewVersionCache(versionCacheFile)

	httpClient := &http.Client{}
	// sha256Fetcher uses context.Background() internally, so SHA256 computation continues even if
	// the request context times out. The result is cached, so the next request gets it immediately.
	sha256Fetcher := NewSHA256Fetcher(httpClient, 10*time.Minute)

	// version_cache.json の SHA256 を Fetcher のインメモリキャッシュに投入
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

	impl := WingetSrcRepositoryImpl{
		packageList:         packageList,
		packageMap:          packageMap,
		versionCache:        versionCache,
		httpClient:          httpClient,
		sha256Fetcher:       sha256Fetcher,
		gracefulDegradation: gracefulDegradation,
	}
	go impl.refreshAll()
	if refreshInterval > 0 {
		impl.startAutoRefresh(ctx, refreshInterval)
	}
	return impl, nil
}
