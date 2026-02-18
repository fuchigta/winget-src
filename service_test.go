package main

import (
	"context"
	"fmt"
	"testing"
)

type mockRepository struct {
	manifests       []Manifest
	packageManifest PackageManifests
	err             error
}

func (m mockRepository) QueryManifest(ctx context.Context, condition QueryManifestCondition) ([]Manifest, error) {
	if m.err != nil {
		return nil, m.err
	}
	result := []Manifest{}
	for _, manifest := range m.manifests {
		entry := PackageListEntry{
			Id:   manifest.PackageIdentifier,
			Name: manifest.PackageName,
		}
		if condition(entry) {
			result = append(result, manifest)
		}
	}
	return result, nil
}

func (m mockRepository) QueryPackageManifests(ctx context.Context, identifier string) (PackageManifests, error) {
	if m.err != nil {
		return PackageManifests{}, m.err
	}
	if m.packageManifest.PackageIdentifier == identifier {
		return m.packageManifest, nil
	}
	return PackageManifests{}, nil
}

func TestInformation(t *testing.T) {
	svc := NewWingetSrcService(mockRepository{}, "api.winget-src")
	info, err := svc.Information(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.SourceIdentifier != "api.winget-src" {
		t.Errorf("expected SourceIdentifier 'api.winget-src', got '%s'", info.SourceIdentifier)
	}
	if len(info.ServerSupportedVersions) != 3 {
		t.Errorf("expected 3 supported versions, got %d", len(info.ServerSupportedVersions))
	}
}

func TestManifestSearch_ByKeyword(t *testing.T) {
	repo := mockRepository{
		manifests: []Manifest{
			{PackageIdentifier: "owner/foo", PackageName: "foo", Publisher: "owner"},
			{PackageIdentifier: "owner/bar", PackageName: "bar", Publisher: "owner"},
		},
	}
	svc := NewWingetSrcService(repo, "api.winget-src")

	res, err := svc.ManifestSearch(context.Background(), ManifestSearchRequest{
		Query: Query{KeyWord: "foo"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("expected 1 result, got %d", len(res))
	}
	if res[0].PackageName != "foo" {
		t.Errorf("expected PackageName 'foo', got '%s'", res[0].PackageName)
	}
}

func TestManifestSearch_ByFilter(t *testing.T) {
	repo := mockRepository{
		manifests: []Manifest{
			{PackageIdentifier: "owner/foo", PackageName: "foo", Publisher: "owner"},
			{PackageIdentifier: "owner/bar", PackageName: "bar", Publisher: "owner"},
		},
	}
	svc := NewWingetSrcService(repo, "api.winget-src")

	res, err := svc.ManifestSearch(context.Background(), ManifestSearchRequest{
		Filters: []FieldQuery{
			{PackageMatchField: PackageMatchFieldPackageIdentifier, RequestMatch: Query{KeyWord: "owner/bar"}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("expected 1 result, got %d", len(res))
	}
	if res[0].PackageIdentifier != "owner/bar" {
		t.Errorf("expected PackageIdentifier 'owner/bar', got '%s'", res[0].PackageIdentifier)
	}
}

func TestManifestSearch_ByInclusion(t *testing.T) {
	repo := mockRepository{
		manifests: []Manifest{
			{PackageIdentifier: "owner/foo", PackageName: "foo", Publisher: "owner"},
			{PackageIdentifier: "owner/bar", PackageName: "bar", Publisher: "owner"},
		},
	}
	svc := NewWingetSrcService(repo, "api.winget-src")

	res, err := svc.ManifestSearch(context.Background(), ManifestSearchRequest{
		Inclusions: []FieldQuery{
			{PackageMatchField: PackageMatchFieldPackageName, RequestMatch: Query{KeyWord: "bar"}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("expected 1 result, got %d", len(res))
	}
	if res[0].PackageName != "bar" {
		t.Errorf("expected PackageName 'bar', got '%s'", res[0].PackageName)
	}
}

func TestManifestSearch_RepositoryError(t *testing.T) {
	repo := mockRepository{err: fmt.Errorf("repository error")}
	svc := NewWingetSrcService(repo, "api.winget-src")

	_, err := svc.ManifestSearch(context.Background(), ManifestSearchRequest{Query: Query{KeyWord: "foo"}})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestPackageManifests_Found(t *testing.T) {
	repo := mockRepository{
		packageManifest: PackageManifests{
			PackageIdentifier: "owner/foo",
			Versions: []PackageManifestsVersion{
				{PackageVersion: "1.0.0"},
				{PackageVersion: "2.0.0"},
			},
		},
	}
	svc := NewWingetSrcService(repo, "api.winget-src")

	res, err := svc.PackageManifests(context.Background(), "owner/foo", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Versions) != 2 {
		t.Errorf("expected 2 versions, got %d", len(res.Versions))
	}
}

func TestPackageManifests_WithVersion(t *testing.T) {
	repo := mockRepository{
		packageManifest: PackageManifests{
			PackageIdentifier: "owner/foo",
			Versions: []PackageManifestsVersion{
				{PackageVersion: "1.0.0"},
				{PackageVersion: "2.0.0"},
			},
		},
	}
	svc := NewWingetSrcService(repo, "api.winget-src")

	res, err := svc.PackageManifests(context.Background(), "owner/foo", "2.0.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Versions) != 1 {
		t.Fatalf("expected 1 version, got %d", len(res.Versions))
	}
	if res.Versions[0].PackageVersion != "2.0.0" {
		t.Errorf("expected version '2.0.0', got '%s'", res.Versions[0].PackageVersion)
	}
}

func TestPackageManifests_VersionNotFound(t *testing.T) {
	repo := mockRepository{
		packageManifest: PackageManifests{
			PackageIdentifier: "owner/foo",
			Versions: []PackageManifestsVersion{
				{PackageVersion: "1.0.0"},
			},
		},
	}
	svc := NewWingetSrcService(repo, "api.winget-src")

	_, err := svc.PackageManifests(context.Background(), "owner/foo", "9.9.9")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestPackageManifests_NotFound(t *testing.T) {
	repo := mockRepository{}
	svc := NewWingetSrcService(repo, "api.winget-src")

	res, err := svc.PackageManifests(context.Background(), "nonexistent", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.PackageIdentifier != "" {
		t.Errorf("expected empty PackageIdentifier, got '%s'", res.PackageIdentifier)
	}
}
