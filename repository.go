package main

import (
	"os"
	"strings"

	"gopkg.in/yaml.v2"
)

type PackageListEntry struct {
	Provider  string `yaml:"provider"`
	Id        string `yaml:"id"`
	Name      string `yaml:"name"`
	Publisher string `yaml:"publisher"`
	Token     string `yaml:"token"`
}

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

		manifests = append(manifests, Manifest{
			PackageIdentifier: entry.Id,
			PackageName:       entry.Name,
			Publisher:         entry.Publisher,
			Versions:          []ManifestVersion{},
		})
	}

	return manifests, nil
}

func (w WingetSrcRepositoryImpl) QueryPackageManifests(identifier string) (PackageManifests, error) {
	condition := ById(identifier)
	var found PackageListEntry
	for _, entry := range w.packageList {
		if condition(entry) {
			found = entry
			break
		}
	}

	if found.Id == "" {
		return PackageManifests{}, nil
	}

	// TODO fetch versions

	return PackageManifests{
		PackageIdentifier: found.Id,
		Versions:          []PackageManifestsVersion{},
	}, nil
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

	// TODO fetch versions

	return WingetSrcRepositoryImpl{
		packageList: packageList,
	}, nil
}
