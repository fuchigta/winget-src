package main

import (
	"context"
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
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "aaaa myapp_windows_x64.zip")
		fmt.Fprintln(w, "bbbb myapp_windows_arm64.zip")
	}))
	defer ts.Close()

	entry := PackageListEntry{
		Name: "myapp",
	}

	releases := []release{
		{
			Name: "v1.0.0",
			Assets: []releaseAsset{
				{Name: "checksums.txt", DownloadUrl: ts.URL, IsChecksum: true},
				{Name: "myapp_windows_x64.zip", DownloadUrl: "https://example.com/myapp_windows_x64.zip"},
				{Name: "myapp_windows_arm64.zip", DownloadUrl: "https://example.com/myapp_windows_arm64.zip"},
				{Name: "myapp_linux_x64.tar.gz", DownloadUrl: "https://example.com/myapp_linux_x64.tar.gz"},
			},
		},
	}

	versions, err := buildVersionsForConfig(context.Background(), ts.Client(), entry, releases, installerConfig{ext: ".zip", installerType: "zip", zipPortable: true}, nil)
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

	versions, err := buildVersionsForConfig(context.Background(), http.DefaultClient, entry, releases, installerConfig{ext: ".zip", installerType: "zip", zipPortable: true}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(versions) != 0 {
		t.Errorf("expected 0 versions, got %d", len(versions))
	}
}

func TestBuildInstallerVersions_Msi(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "cccc myapp_windows_x64.msi")
		fmt.Fprintln(w, "dddd myapp_windows_x86.msi")
	}))
	defer ts.Close()

	entry := PackageListEntry{Name: "myapp"}

	releases := []release{
		{
			Name: "v2.0.0",
			Assets: []releaseAsset{
				{Name: "checksums.txt", DownloadUrl: ts.URL, IsChecksum: true},
				{Name: "myapp_windows_x64.msi", DownloadUrl: "https://example.com/myapp_windows_x64.msi"},
				{Name: "myapp_windows_x86.msi", DownloadUrl: "https://example.com/myapp_windows_x86.msi"},
			},
		},
	}

	versions, err := buildVersionsForConfig(context.Background(), ts.Client(), entry, releases, installerConfig{ext: ".msi", installerType: "msi"}, nil)
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

	versions, err := buildVersionsForConfig(context.Background(), http.DefaultClient, entry, releases, installerConfig{ext: ".exe", installerType: "exe"}, nil)
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

	checksums, err := fetchChecksums(context.Background(), ts.Client(), ts.URL)
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

	_, err := fetchChecksums(context.Background(), ts.Client(), ts.URL)
	if err == nil {
		t.Fatal("expected error for non-200 status")
	}
}

func TestFetchChecksums_BadFormat(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "invalid-line-without-two-fields")
	}))
	defer ts.Close()

	_, err := fetchChecksums(context.Background(), ts.Client(), ts.URL)
	if err == nil {
		t.Fatal("expected error for bad checksum format")
	}
}

func TestCollectChecksums_MultipleAssets(t *testing.T) {
	ts1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "abc123 myapp_windows_x64.zip")
	}))
	defer ts1.Close()

	ts2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "def456 myapp_windows_arm64.zip")
	}))
	defer ts2.Close()

	assets := []releaseAsset{
		{Name: "checksums_x64.txt", DownloadUrl: ts1.URL, IsChecksum: true},
		{Name: "checksums_arm64.txt", DownloadUrl: ts2.URL, IsChecksum: true},
		{Name: "myapp_windows_x64.zip", DownloadUrl: "https://example.com/myapp_windows_x64.zip", IsChecksum: false},
	}

	checksums, err := collectChecksums(context.Background(), ts1.Client(), assets)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(checksums) != 2 {
		t.Fatalf("expected 2 checksums, got %d", len(checksums))
	}
	if checksums["myapp_windows_x64.zip"] != "abc123" {
		t.Errorf("unexpected checksum for x64: %s", checksums["myapp_windows_x64.zip"])
	}
	if checksums["myapp_windows_arm64.zip"] != "def456" {
		t.Errorf("unexpected checksum for arm64: %s", checksums["myapp_windows_arm64.zip"])
	}
}

func TestCollectChecksums_NoChecksumAssets(t *testing.T) {
	assets := []releaseAsset{
		{Name: "myapp_windows_x64.zip", DownloadUrl: "https://example.com/myapp_windows_x64.zip", IsChecksum: false},
	}

	checksums, err := collectChecksums(context.Background(), http.DefaultClient, assets)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(checksums) != 0 {
		t.Errorf("expected 0 checksums, got %d", len(checksums))
	}
}

func TestBuildZipPortableVersions_CustomScope(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "aaaa myapp_windows_x64.zip")
	}))
	defer ts.Close()

	entry := PackageListEntry{
		Name:  "myapp",
		Scope: "machine",
	}

	releases := []release{
		{
			Name: "v1.0.0",
			Assets: []releaseAsset{
				{Name: "checksums.txt", DownloadUrl: ts.URL, IsChecksum: true},
				{Name: "myapp_windows_x64.zip", DownloadUrl: "https://example.com/myapp_windows_x64.zip"},
			},
		},
	}

	versions, err := buildVersionsForConfig(context.Background(), ts.Client(), entry, releases, installerConfig{ext: ".zip", installerType: "zip", zipPortable: true}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(versions) != 1 || len(versions[0].Installers) != 1 {
		t.Fatalf("expected 1 version with 1 installer, got %d versions", len(versions))
	}

	if versions[0].Installers[0].Scope != "machine" {
		t.Errorf("expected Scope 'machine', got '%s'", versions[0].Installers[0].Scope)
	}
}

func TestBuildInstallerVersions_CustomScope(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "cccc myapp_windows_x64.msi")
	}))
	defer ts.Close()

	entry := PackageListEntry{
		Name:  "myapp",
		Scope: "machine",
	}

	releases := []release{
		{
			Name: "v2.0.0",
			Assets: []releaseAsset{
				{Name: "checksums.txt", DownloadUrl: ts.URL, IsChecksum: true},
				{Name: "myapp_windows_x64.msi", DownloadUrl: "https://example.com/myapp_windows_x64.msi"},
			},
		},
	}

	versions, err := buildVersionsForConfig(context.Background(), ts.Client(), entry, releases, installerConfig{ext: ".msi", installerType: "msi"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(versions) != 1 || len(versions[0].Installers) != 1 {
		t.Fatalf("expected 1 version with 1 installer, got %d versions", len(versions))
	}

	if versions[0].Installers[0].Scope != "machine" {
		t.Errorf("expected Scope 'machine', got '%s'", versions[0].Installers[0].Scope)
	}
}

func TestComputeSHA256FromURL(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello"))
	}))
	defer ts.Close()

	hash, err := computeSHA256FromURL(context.Background(), ts.Client(), ts.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hash) != 64 {
		t.Errorf("expected 64-char hex SHA256, got %d chars: %s", len(hash), hash)
	}
}

func TestBuildVersions_FallbackSHA256(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("fake installer"))
	}))
	defer ts.Close()

	entry := PackageListEntry{Name: "myapp"}
	releases := []release{
		{
			Name: "v1.0.0",
			Assets: []releaseAsset{
				{Name: "myapp_windows_x64.msi", DownloadUrl: ts.URL + "/myapp_windows_x64.msi"},
			},
		},
	}

	versions, err := buildVersionsForConfig(context.Background(), ts.Client(), entry, releases, installerConfig{ext: ".msi", installerType: "msi"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(versions) != 1 || len(versions[0].Installers) != 1 {
		t.Fatalf("expected 1 version with 1 installer")
	}

	sha := versions[0].Installers[0].InstallerSha256
	if sha == "" {
		t.Error("expected non-empty InstallerSha256 from fallback computation")
	}
	if len(sha) != 64 {
		t.Errorf("expected 64-char SHA256, got %d chars: %s", len(sha), sha)
	}
}

func TestCollectChecksums_FetchError(t *testing.T) {
	assets := []releaseAsset{
		{Name: "checksums.txt", DownloadUrl: "http://127.0.0.1:0/nonexistent", IsChecksum: true},
	}

	_, err := collectChecksums(context.Background(), http.DefaultClient, assets)
	if err == nil {
		t.Fatal("expected error when fetching checksums fails")
	}
}
