package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDetectArch(t *testing.T) {
	tests := []struct {
		name     string
		wantArch string
		wantOk   bool
	}{
		{"app_windows_x86_64.zip", "x64", true},
		{"app_windows_x64.zip", "x64", true},
		{"app_windows_amd64.zip", "x64", true},
		{"app_windows_i386.zip", "x86", true},
		{"app_windows_x86.zip", "x86", true},
		{"app_windows_arm64.zip", "arm64", true},
		{"app_windows_aarch64.zip", "arm64", true},
		{"app_linux_x64.tar.gz", "x64", true},
		{"app_windows.zip", "", false},
		{"app.zip", "", false},
	}

	for _, tt := range tests {
		arch, ok := detectArch(tt.name)
		if arch != tt.wantArch || ok != tt.wantOk {
			t.Errorf("detectArch(%q) = (%q, %v), want (%q, %v)", tt.name, arch, ok, tt.wantArch, tt.wantOk)
		}
	}
}

func TestBuildZipPortableVersions(t *testing.T) {
	entry := PackageListEntry{
		Name: "myapp",
	}

	releases := []release{
		{
			Name: "v1.0.0",
			Assets: []releaseAsset{
				{Name: "myapp_windows_x64.zip", DownloadUrl: "https://example.com/myapp_windows_x64.zip"},
				{Name: "myapp_windows_arm64.zip", DownloadUrl: "https://example.com/myapp_windows_arm64.zip"},
				{Name: "myapp_linux_x64.tar.gz", DownloadUrl: "https://example.com/myapp_linux_x64.tar.gz"},
			},
		},
	}

	versions, err := buildZipPortableVersions(http.DefaultClient, entry, releases)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(versions) != 1 {
		t.Fatalf("expected 1 version, got %d", len(versions))
	}

	if versions[0].Version != "v1.0.0" {
		t.Errorf("expected version 'v1.0.0', got '%s'", versions[0].Version)
	}

	if len(versions[0].Installers) != 2 {
		t.Fatalf("expected 2 installers, got %d", len(versions[0].Installers))
	}

	for _, inst := range versions[0].Installers {
		if inst.InstallerType != "zip" {
			t.Errorf("expected InstallerType 'zip', got '%s'", inst.InstallerType)
		}
		if inst.NestedInstallerType != "portable" {
			t.Errorf("expected NestedInstallerType 'portable', got '%s'", inst.NestedInstallerType)
		}
		if len(inst.NestedInstallerFiles) != 1 || inst.NestedInstallerFiles[0].RelativeFilePath != "myapp.exe" {
			t.Errorf("expected NestedInstallerFiles with 'myapp.exe', got %v", inst.NestedInstallerFiles)
		}
	}
}

func TestBuildZipPortableVersions_NoMatch(t *testing.T) {
	entry := PackageListEntry{Name: "myapp"}

	releases := []release{
		{
			Name: "v1.0.0",
			Assets: []releaseAsset{
				{Name: "myapp_linux_x64.tar.gz", DownloadUrl: "https://example.com/myapp_linux_x64.tar.gz"},
			},
		},
	}

	versions, err := buildZipPortableVersions(http.DefaultClient, entry, releases)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(versions) != 0 {
		t.Errorf("expected 0 versions, got %d", len(versions))
	}
}

func TestBuildInstallerVersions_Msi(t *testing.T) {
	entry := PackageListEntry{Name: "myapp"}

	releases := []release{
		{
			Name: "v2.0.0",
			Assets: []releaseAsset{
				{Name: "myapp_windows_x64.msi", DownloadUrl: "https://example.com/myapp_windows_x64.msi"},
				{Name: "myapp_windows_x86.msi", DownloadUrl: "https://example.com/myapp_windows_x86.msi"},
			},
		},
	}

	versions, err := buildInstallerVersions(http.DefaultClient, entry, releases, "msi", ".msi")
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
		if inst.InstallerType != "msi" {
			t.Errorf("expected InstallerType 'msi', got '%s'", inst.InstallerType)
		}
		if inst.NestedInstallerType != "" {
			t.Errorf("expected empty NestedInstallerType, got '%s'", inst.NestedInstallerType)
		}
	}
}

func TestBuildInstallerVersions_NoMatch(t *testing.T) {
	entry := PackageListEntry{Name: "myapp"}

	releases := []release{
		{
			Name: "v1.0.0",
			Assets: []releaseAsset{
				{Name: "myapp_windows.zip", DownloadUrl: "https://example.com/myapp_windows.zip"},
			},
		},
	}

	versions, err := buildInstallerVersions(http.DefaultClient, entry, releases, "exe", ".exe")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(versions) != 0 {
		t.Errorf("expected 0 versions, got %d", len(versions))
	}
}

func TestFetchChecksums_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "abc123 myapp_windows_x64.zip")
		fmt.Fprintln(w, "def456 myapp_windows_arm64.zip")
	}))
	defer ts.Close()

	checksums, err := fetchChecksums(ts.Client(), ts.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(checksums) != 2 {
		t.Fatalf("expected 2 checksums, got %d", len(checksums))
	}

	if checksums["myapp_windows_x64.zip"] != "abc123" {
		t.Errorf("expected checksum 'abc123' for myapp_windows_x64.zip, got '%s'", checksums["myapp_windows_x64.zip"])
	}

	if checksums["myapp_windows_arm64.zip"] != "def456" {
		t.Errorf("expected checksum 'def456' for myapp_windows_arm64.zip, got '%s'", checksums["myapp_windows_arm64.zip"])
	}
}

func TestFetchChecksums_BadStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintln(w, "not found")
	}))
	defer ts.Close()

	_, err := fetchChecksums(ts.Client(), ts.URL)
	if err == nil {
		t.Fatal("expected error for non-200 status")
	}
}

func TestFetchChecksums_BadFormat(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "invalid-line-without-two-fields")
	}))
	defer ts.Close()

	_, err := fetchChecksums(ts.Client(), ts.URL)
	if err == nil {
		t.Fatal("expected error for bad checksum format")
	}
}
