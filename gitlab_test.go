package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGitlab_FetchVersions_ZipPortable(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		releases := []gitlabRelease{
			{
				Name: "v1.0.0",
				Assets: gitlabAssets{
					Links: []gitlabAssetLink{
						{
							Name:     "myapp_windows_x64.zip",
							Url:      "https://example.com/myapp_windows_x64.zip",
							LinkType: "package",
						},
						{
							Name:     "myapp_linux_x64.tar.gz",
							Url:      "https://example.com/myapp_linux_x64.tar.gz",
							LinkType: "package",
						},
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(releases)
	}))
	defer ts.Close()

	g := Gitlab{httpClient: ts.Client(), baseURL: ts.URL}

	entry := PackageListEntry{
		ProjectID:     42,
		Name:          "myapp",
		InstallerType: InstallerTypeZipPortable,
	}

	versions, err := g.FetchVersions(context.Background(), entry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(versions) != 1 {
		t.Fatalf("expected 1 version, got %d", len(versions))
	}

	if versions[0].Version != "v1.0.0" {
		t.Errorf("expected version 'v1.0.0', got '%s'", versions[0].Version)
	}

	if len(versions[0].Installers) != 1 {
		t.Fatalf("expected 1 installer, got %d", len(versions[0].Installers))
	}

	if versions[0].Installers[0].Architecture != "x64" {
		t.Errorf("expected architecture 'x64', got '%s'", versions[0].Installers[0].Architecture)
	}
}

func TestGitlab_FetchVersions_Exe(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		releases := []gitlabRelease{
			{
				Name: "v3.0.0",
				Assets: gitlabAssets{
					Links: []gitlabAssetLink{
						{
							Name:     "myapp_windows_x64.exe",
							Url:      "https://example.com/myapp_windows_x64.exe",
							LinkType: "package",
						},
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(releases)
	}))
	defer ts.Close()

	g := Gitlab{httpClient: ts.Client(), baseURL: ts.URL}

	entry := PackageListEntry{
		ProjectID:     42,
		Name:          "myapp",
		InstallerType: InstallerTypeExe,
	}

	versions, err := g.FetchVersions(context.Background(), entry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(versions) != 1 {
		t.Fatalf("expected 1 version, got %d", len(versions))
	}

	if versions[0].Installers[0].InstallerType != InstallerTypeExe {
		t.Errorf("expected InstallerType '%s', got '%s'", InstallerTypeExe, versions[0].Installers[0].InstallerType)
	}
}

func TestGitlab_FetchVersions_ErrorResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"message":"403 Forbidden"}`))
	}))
	defer ts.Close()

	g := Gitlab{httpClient: ts.Client(), baseURL: ts.URL}

	entry := PackageListEntry{
		ProjectID:     42,
		Name:          "myapp",
		InstallerType: InstallerTypeZipPortable,
	}

	_, err := g.FetchVersions(context.Background(), entry)
	if err == nil {
		t.Fatal("expected error for non-200 status")
	}
}

func TestGitlab_FetchVersions_WithToken(t *testing.T) {
	var gotToken string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.Header.Get("PRIVATE-TOKEN")
		releases := []gitlabRelease{}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(releases)
	}))
	defer ts.Close()

	g := Gitlab{httpClient: ts.Client(), baseURL: ts.URL}

	entry := PackageListEntry{
		ProjectID:     42,
		Name:          "myapp",
		Token:         "mysecrettoken",
		InstallerType: InstallerTypeZipPortable,
	}

	_, err := g.FetchVersions(context.Background(), entry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotToken != "mysecrettoken" {
		t.Errorf("expected PRIVATE-TOKEN 'mysecrettoken', got '%s'", gotToken)
	}
}

func TestGitlab_FetchVersions_UnknownInstallerType(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		releases := []gitlabRelease{{Name: "v1.0.0"}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(releases)
	}))
	defer ts.Close()

	g := Gitlab{httpClient: ts.Client(), baseURL: ts.URL}

	entry := PackageListEntry{
		ProjectID:     42,
		Name:          "myapp",
		InstallerType: "unknown-type",
	}

	_, err := g.FetchVersions(context.Background(), entry)
	if err == nil {
		t.Fatal("expected error for unknown installer type")
	}
}

func TestGitlab_FetchVersions_FallbackToEndpoint(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		releases := []gitlabRelease{}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(releases)
	}))
	defer ts.Close()

	// baseURLを指定せず、entry.Endpointを使用
	g := Gitlab{httpClient: ts.Client()}

	entry := PackageListEntry{
		Endpoint:      ts.URL,
		ProjectID:     42,
		Name:          "myapp",
		InstallerType: InstallerTypeZipPortable,
	}

	_, err := g.FetchVersions(context.Background(), entry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
