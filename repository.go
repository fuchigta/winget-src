package main

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v2"
)

type QueryManifestConditon func(PackageListEntry) bool

type WingetSrcRepository interface {
	QueryManifest(condition QueryManifestConditon) ([]Manifest, error)
	QueryPackageManifests(identifier string) (PackageManifests, error)
}

type WingetSrcRepositoryImpl struct {
	packageList []PackageListEntry
}

func ById(id string) QueryManifestConditon {
	return func(entry PackageListEntry) bool {
		return strings.Contains(entry.Id, id)
	}
}

func ByName(name string) QueryManifestConditon {
	return func(entry PackageListEntry) bool {
		return strings.Contains(entry.Name, name)
	}
}

func Or(conditions ...QueryManifestConditon) QueryManifestConditon {
	return func(entry PackageListEntry) bool {
		for _, condition := range conditions {
			if condition(entry) {
				return true
			}
		}

		return false
	}
}

func And(conditions ...QueryManifestConditon) QueryManifestConditon {
	return func(entry PackageListEntry) bool {
		for _, condition := range conditions {
			if !condition(entry) {
				return false
			}
		}

		return true
	}
}

func (w WingetSrcRepositoryImpl) QueryManifest(condition QueryManifestConditon) ([]Manifest, error) {
	manifests := []Manifest{}

	for _, entry := range w.packageList {
		if !condition(entry) {
			continue
		}

		provider, err := dispatchProvider(entry)
		if err != nil {
			return nil, fmt.Errorf("unknown package provider")
		}

		versions, err := provider.FetchVersions(entry)
		if err != nil {
			return nil, fmt.Errorf("fetch versions: %w", err)
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
		if entry.Id == identifier {
			found = entry
			break
		}
	}

	if found.Id == "" {
		return PackageManifests{}, fmt.Errorf("unknown package identifier")
	}

	provider, err := dispatchProvider(found)
	if err != nil {
		return PackageManifests{}, fmt.Errorf("unknown package provider")
	}

	versions, err := provider.FetchVersions(found)
	if err != nil {
		return PackageManifests{}, fmt.Errorf("fetch versions: %w", err)
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
		packageList: packageList,
	}, nil
}
