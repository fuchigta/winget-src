package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// rewriteTransport rewrites all requests to the target server, preserving the path.
type rewriteTransport struct {
	target string
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	u, _ := url.Parse(t.target)
	req2 := req.Clone(req.Context())
	req2.URL.Scheme = u.Scheme
	req2.URL.Host = u.Host
	return http.DefaultTransport.RoundTrip(req2)
}

func newMockHTTPClient(ts *httptest.Server) *http.Client {
	return &http.Client{Transport: &rewriteTransport{target: ts.URL}}
}

func TestCheckPackage_ReleasesWithMatchingAssets(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		releases := []map[string]interface{}{
			{
				"tag_name": "v1.0.0",
				"assets": []map[string]interface{}{
					{"name": "app_windows_x64.zip", "browser_download_url": "https://example.com/app_windows_x64.zip", "content_type": "application/zip"},
				},
			},
		}
		json.NewEncoder(w).Encode(releases)
	}))
	defer ts.Close()

	entry := PackageListEntry{
		Provider:      "github",
		Id:            "owner/repo",
		Name:          "Test",
		Publisher:     "Test",
		InstallerType: InstallerTypeZipPortable,
	}

	err := checkPackage(context.Background(), newMockHTTPClient(ts), entry, false)
	if err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestCheckPackage_ReleasesWithNoMatchingAssets(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		releases := []map[string]interface{}{
			{
				"tag_name": "v1.0.0",
				"assets": []map[string]interface{}{
					{"name": "app_linux_x64.tar.gz", "browser_download_url": "https://example.com/app_linux_x64.tar.gz", "content_type": "application/gzip"},
				},
			},
		}
		json.NewEncoder(w).Encode(releases)
	}))
	defer ts.Close()

	entry := PackageListEntry{
		Provider:      "github",
		Id:            "owner/repo",
		InstallerType: InstallerTypeZipPortable,
	}

	err := checkPackage(context.Background(), newMockHTTPClient(ts), entry, false)
	if err == nil {
		t.Error("expected error when no assets match installer type")
	}
}

func TestCheckPackage_NoReleasesDefaultNG(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]interface{}{})
	}))
	defer ts.Close()

	entry := PackageListEntry{
		Provider:      "github",
		Id:            "owner/repo",
		InstallerType: InstallerTypeZipPortable,
	}

	err := checkPackage(context.Background(), newMockHTTPClient(ts), entry, false)
	if err == nil {
		t.Error("expected error when no releases and allowNoReleases=false")
	}
}

func TestCheckPackage_NoReleasesAllowNoReleases(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]interface{}{})
	}))
	defer ts.Close()

	entry := PackageListEntry{
		Provider:      "github",
		Id:            "owner/repo",
		InstallerType: InstallerTypeZipPortable,
	}

	err := checkPackage(context.Background(), newMockHTTPClient(ts), entry, true)
	if err != nil {
		t.Errorf("expected no error when allowNoReleases=true, got: %v", err)
	}
}

func TestCheckPackage_UnknownInstallerType(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		releases := []map[string]interface{}{
			{"tag_name": "v1.0.0", "assets": []interface{}{}},
		}
		json.NewEncoder(w).Encode(releases)
	}))
	defer ts.Close()

	entry := PackageListEntry{
		Provider:      "github",
		Id:            "owner/repo",
		InstallerType: "unknown-type",
	}

	err := checkPackage(context.Background(), newMockHTTPClient(ts), entry, false)
	if err == nil {
		t.Error("expected error for unknown installer type")
	}
}
