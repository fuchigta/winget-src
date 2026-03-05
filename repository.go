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
	QueryPackageManifests(ctx context.Context, identifier string) (PackageManifests, error)
}

type WingetSrcRepositoryImpl struct {
	packageList         []PackageListEntry
	packageMap          map[string]PackageListEntry
	versionCache        *Cache[[]Version]
	httpClient          *http.Client
	downloadClient      *http.Client
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

func (w WingetSrcRepositoryImpl) fetchVersionsCached(ctx context.Context, entry PackageListEntry) ([]Version, error) {
	if cached, ok := w.versionCache.Get(entry.Id); ok {
		return cached, nil
	}

	provider, err := dispatchProvider(entry, w.httpClient, w.downloadClient)
	if err != nil {
		return nil, err
	}

	const maxAttempts = 3
	backoff := time.Second

	var versions []Version
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		start := time.Now()
		versions, err = provider.FetchVersions(ctx, entry)
		elapsed := time.Since(start)

		if err == nil {
			slog.Debug("provider fetch succeeded",
				"package", entry.Id,
				"provider", entry.Provider,
				"latency_ms", elapsed.Milliseconds(),
				"versions", len(versions),
			)
			break
		}

		slog.Warn("provider fetch failed",
			"package", entry.Id,
			"provider", entry.Provider,
			"latency_ms", elapsed.Milliseconds(),
			"attempt", attempt,
			"error", err,
		)

		if attempt == maxAttempts {
			return nil, fmt.Errorf("fetch versions (after %d attempts): %w", maxAttempts, err)
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}

		if backoff < 8*time.Second {
			backoff *= 2
		}
	}

	w.versionCache.Set(entry.Id, versions)
	return versions, nil
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
			versions, err := w.fetchVersionsCached(ctx, entry)
			if err != nil {
				results[i] = result{index: i, err: err}
				return
			}
			manifestVersions := []ManifestVersion{}
			for _, version := range versions {
				manifestVersions = append(manifestVersions, ManifestVersion{
					PackageVersion: version.Version,
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
				slog.Warn("failed to fetch versions, skipping package", "package", matched[r.index].Id, "error", r.err)
				continue
			}
			return nil, r.err
		}
		manifests = append(manifests, r.manifest)
	}

	return manifests, nil
}

func (w WingetSrcRepositoryImpl) QueryPackageManifests(ctx context.Context, identifier string) (PackageManifests, error) {
	found, ok := w.packageMap[strings.ToLower(identifier)]
	if !ok {
		return PackageManifests{}, nil
	}

	versions, err := w.fetchVersionsCached(ctx, found)
	if err != nil {
		return PackageManifests{}, err
	}

	pkgManifestVersions := []PackageManifestsVersion{}

	for _, version := range versions {
		pkgManifestVersions = append(pkgManifestVersions, PackageManifestsVersion{
			PackageVersion: version.Version,
			Installers:     version.Installers,
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

func dispatchProvider(entry PackageListEntry, httpClient *http.Client, downloadClient *http.Client) (PackageProvider, error) {
	switch entry.Provider {
	case "github":
		return Github{httpClient: httpClient, downloadClient: downloadClient}, nil
	case "gitlab":
		return Gitlab{httpClient: httpClient, downloadClient: downloadClient}, nil
	default:
		return nil, fmt.Errorf("unknown package provider: %s", entry.Provider)
	}
}

func NewWingetSrcRepository(ctx context.Context, packageListPath string, cacheTTL time.Duration, cacheCleanupInterval time.Duration, httpClientTimeout time.Duration, gracefulDegradation bool, cacheMaxEntries int) (WingetSrcRepository, error) {
	f, err := os.Open(packageListPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var packageList []PackageListEntry
	if err := yaml.NewDecoder(f).Decode(&packageList); err != nil {
		return nil, err
	}

	packageMap := make(map[string]PackageListEntry, len(packageList))
	for _, entry := range packageList {
		packageMap[strings.ToLower(entry.PackageIdentifier())] = entry
	}

	cache := NewCache[[]Version](cacheTTL, cacheMaxEntries)
	cache.StartCleanup(ctx, cacheCleanupInterval)

	httpClient := &http.Client{Timeout: httpClientTimeout}
	// downloadClient has no timeout; large asset downloads are bounded by the handler context timeout.
	downloadClient := &http.Client{}

	return WingetSrcRepositoryImpl{
		packageList:         packageList,
		packageMap:          packageMap,
		versionCache:        cache,
		httpClient:          httpClient,
		downloadClient:      downloadClient,
		gracefulDegradation: gracefulDegradation,
	}, nil
}
