package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGithub_FetchVersions_ZipPortable(t *testing.T) {
	var tsURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/checksums.txt" {
			w.Write([]byte("aaaa myapp_windows_x64.zip\n"))
			return
		}
		releases := []githubRelease{
			{
				TagName: "v1.0.0",
				Assets: []githubAsset{
					{
						Name:               "checksums.txt",
						BrowserDownloadUrl: tsURL + "/checksums.txt",
						ContentType:        "text/plain",
					},
					{
						Name:               "myapp_windows_x64.zip",
						BrowserDownloadUrl: "https://example.com/myapp_windows_x64.zip",
						ContentType:        "application/zip",
					},
					{
						Name:               "myapp_linux_x64.tar.gz",
						BrowserDownloadUrl: "https://example.com/myapp_linux_x64.tar.gz",
						ContentType:        "application/gzip",
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(releases)
	}))
	tsURL = ts.URL
	defer ts.Close()

	g := Github{httpClient: ts.Client(), baseURL: ts.URL}

	entry := PackageListEntry{
		Id:            "owner/myapp",
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

	if versions[0].Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got '%s'", versions[0].Version)
	}

	if len(versions[0].Installers) != 1 {
		t.Fatalf("expected 1 installer, got %d", len(versions[0].Installers))
	}

	if versions[0].Installers[0].Architecture != "x64" {
		t.Errorf("expected architecture 'x64', got '%s'", versions[0].Installers[0].Architecture)
	}
}

func TestGithub_FetchVersions_Msi(t *testing.T) {
	var tsURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/checksums.txt" {
			w.Write([]byte("cccc myapp_windows_x64.msi\ndddd myapp_windows_x86.msi\n"))
			return
		}
		releases := []githubRelease{
			{
				TagName: "v2.0.0",
				Assets: []githubAsset{
					{
						Name:               "checksums.txt",
						BrowserDownloadUrl: tsURL + "/checksums.txt",
						ContentType:        "text/plain",
					},
					{
						Name:               "myapp_windows_x64.msi",
						BrowserDownloadUrl: "https://example.com/myapp_windows_x64.msi",
						ContentType:        "application/octet-stream",
					},
					{
						Name:               "myapp_windows_x86.msi",
						BrowserDownloadUrl: "https://example.com/myapp_windows_x86.msi",
						ContentType:        "application/octet-stream",
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(releases)
	}))
	tsURL = ts.URL
	defer ts.Close()

	g := Github{httpClient: ts.Client(), baseURL: ts.URL}

	entry := PackageListEntry{
		Id:            "owner/myapp",
		Name:          "myapp",
		InstallerType: InstallerTypeMsi,
	}

	versions, err := g.FetchVersions(context.Background(), entry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(versions) != 1 {
		t.Fatalf("expected 1 version, got %d", len(versions))
	}

	if len(versions[0].Installers) != 2 {
		t.Fatalf("expected 2 installers, got %d", len(versions[0].Installers))
	}

	for _, inst := range versions[0].Installers {
		if inst.InstallerType != InstallerTypeMsi {
			t.Errorf("expected InstallerType '%s', got '%s'", InstallerTypeMsi, inst.InstallerType)
		}
	}
}

func TestGithub_FetchVersions_ErrorResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message":"Bad credentials"}`))
	}))
	defer ts.Close()

	g := Github{httpClient: ts.Client(), baseURL: ts.URL}

	entry := PackageListEntry{
		Id:            "owner/myapp",
		Name:          "myapp",
		InstallerType: InstallerTypeZipPortable,
	}

	_, err := g.FetchVersions(context.Background(), entry)
	if err == nil {
		t.Fatal("expected error for non-200 status")
	}
}

func TestGithub_FetchVersions_UnknownInstallerType(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		releases := []githubRelease{{TagName: "v1.0.0"}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(releases)
	}))
	defer ts.Close()

	g := Github{httpClient: ts.Client(), baseURL: ts.URL}

	entry := PackageListEntry{
		Id:            "owner/myapp",
		Name:          "myapp",
		InstallerType: "unknown-type",
	}

	_, err := g.FetchVersions(context.Background(), entry)
	if err == nil {
		t.Fatal("expected error for unknown installer type")
	}
}

func TestGithub_FetchVersions_WithToken(t *testing.T) {
	var gotToken string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.Header.Get("Authorization")
		releases := []githubRelease{}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(releases)
	}))
	defer ts.Close()

	g := Github{httpClient: ts.Client(), baseURL: ts.URL}

	entry := PackageListEntry{
		Id:            "owner/myapp",
		Name:          "myapp",
		Token:         "mytoken",
		InstallerType: InstallerTypeZipPortable,
	}

	_, err := g.FetchVersions(context.Background(), entry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotToken != "token mytoken" {
		t.Errorf("expected Authorization header 'token mytoken', got '%s'", gotToken)
	}
}
