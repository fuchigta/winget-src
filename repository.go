package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
)

type QueryManifestCondition func(PackageListEntry) bool

type WingetSrcRepository interface {
	QueryManifest(condition QueryManifestCondition) ([]Manifest, error)
	QueryPackageManifests(identifier string) (PackageManifests, error)
}

type WingetSrcRepositoryImpl struct {
	packageList   []PackageListEntry
	versionCache  *Cache[[]Version]
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

func (w WingetSrcRepositoryImpl) fetchVersionsCached(entry PackageListEntry) ([]Version, error) {
	if cached, ok := w.versionCache.Get(entry.Id); ok {
		return cached, nil
	}

	provider, err := dispatchProvider(entry)
	if err != nil {
		return nil, fmt.Errorf("unknown package provider")
	}

	versions, err := provider.FetchVersions(entry)
	if err != nil {
		return nil, fmt.Errorf("fetch versions: %w", err)
	}

	w.versionCache.Set(entry.Id, versions)
	return versions, nil
}

func (w WingetSrcRepositoryImpl) QueryManifest(condition QueryManifestCondition) ([]Manifest, error) {
	manifests := []Manifest{}

	for _, entry := range w.packageList {
		if !condition(entry) {
			continue
		}

		versions, err := w.fetchVersionsCached(entry)
		if err != nil {
			return nil, err
		}

		manifestVersions := []ManifestVersion{}

		for _, version := range versions {
			manifestVersions = append(manifestVersions, ManifestVersion{
				PackageVersion: version.Version,
			})
		}

		manifests = append(manifests, Manifest{
			PackageIdentifier: entry.Id,
			PackageName:       entry.Name,
			Publisher:         entry.Publisher,
			Versions:          manifestVersions,
		})
	}

	return manifests, nil
}

func (w WingetSrcRepositoryImpl) QueryPackageManifests(identifier string) (PackageManifests, error) {
	var found PackageListEntry
	for _, entry := range w.packageList {
		if strings.EqualFold(entry.Id, identifier) {
			found = entry
			break
		}
	}

	if found.Id == "" {
		return PackageManifests{}, nil
	}

	versions, err := w.fetchVersionsCached(found)
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
				PackageLocale:    "en-us",
				Publisher:        found.Publisher,
				ShortDescription: found.Description,
			},
		})
	}

	return PackageManifests{
		PackageIdentifier: found.Id,
		Versions:          pkgManifestVersions,
	}, nil
}

func dispatchProvider(entry PackageListEntry) (PackageProvider, error) {
	switch entry.Provider {
	case "github":
		return Github{}, nil
	case "gitlab":
		return Gitlab{}, nil
	default:
		return nil, fmt.Errorf("unknown package provider")
	}
}

func NewWingetSrcRepository(packageListPath string) (WingetSrcRepository, error) {
	f, err := os.Open(packageListPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var packageList []PackageListEntry
	if err := yaml.NewDecoder(f).Decode(&packageList); err != nil {
		return nil, err
	}

	return WingetSrcRepositoryImpl{
		packageList:  packageList,
		versionCache: NewCache[[]Version](5 * time.Minute),
	}, nil
}
