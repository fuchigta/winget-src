package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newE2ERepo builds a WingetSrcRepositoryImpl backed by a mock GitHub server.
func newE2ERepo(t *testing.T, githubServer *httptest.Server, entries []PackageListEntry) WingetSrcRepositoryImpl {
	t.Helper()
	cache := NewCache[[]Version](5*time.Minute, 0)
	cache.StartCleanup(context.Background(), 10*time.Minute)

	packageMap := make(map[string]PackageListEntry, len(entries))
	for _, e := range entries {
		packageMap[strings.ToLower(e.Id)] = e
	}

	return WingetSrcRepositoryImpl{
		packageList:         entries,
		packageMap:          packageMap,
		versionCache:        cache,
		httpClient:          githubServer.Client(),
		gracefulDegradation: false,
	}
}

func newE2EHandler(svc WingetSrcService) http.Handler {
	return NewWingetSrcHandler(svc, 10*time.Second)
}

func TestE2E_ManifestSearch_Then_PackageManifests(t *testing.T) {
	// GitHub API mock: returns a single release with a Windows x64 zip asset
	githubMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		releases := []githubRelease{
			{
				Name: "v1.2.3",
				Assets: []githubAsset{
					{
						Name:               "myapp_windows_x64.zip",
						BrowserDownloadUrl: "https://example.com/myapp_windows_x64.zip",
						ContentType:        "application/zip",
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(releases); err != nil {
			t.Errorf("mock: encode error: %v", err)
		}
	}))
	defer githubMock.Close()

	// entry.Id は owner/repo 形式（GitHub API パスと WinGet identifier を兼ねる）
	entry := PackageListEntry{
		Provider:      "github",
		Id:            "owner/myapp",
		Name:          "myapp",
		Publisher:     "owner",
		InstallerType: InstallerTypeZipPortable,
	}

	repo := newE2ERepo(t, githubMock, []PackageListEntry{entry})

	// Pre-populate cache using the mock GitHub server directly (bypasses dispatchProvider)
	versions, err := Github{httpClient: githubMock.Client(), baseURL: githubMock.URL}.FetchVersions(context.Background(), entry)
	if err != nil {
		t.Fatalf("FetchVersions: %v", err)
	}
	repo.versionCache.Set(entry.Id, versions)

	svc := NewWingetSrcService(repo, "api.winget-src")
	handler := newE2EHandler(svc)

	// Step 1: POST /manifestSearch
	searchBody, _ := json.Marshal(ManifestSearchRequest{
		Query: Query{KeyWord: "myapp"},
	})
	req1 := httptest.NewRequest(http.MethodPost, "/manifestSearch", bytes.NewReader(searchBody))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Fatalf("manifestSearch: expected 200, got %d; body: %s", w1.Code, w1.Body.String())
	}

	var searchResp DataResponse
	if err := json.NewDecoder(w1.Body).Decode(&searchResp); err != nil {
		t.Fatalf("manifestSearch: decode response: %v", err)
	}

	// Step 2: GET /packageManifests/owner/myapp（スラッシュを含む identifier）
	req2 := httptest.NewRequest(http.MethodGet, "/packageManifests/owner/myapp", nil)
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("packageManifests: expected 200, got %d; body: %s", w2.Code, w2.Body.String())
	}

	var manifestResp DataResponse
	if err := json.NewDecoder(w2.Body).Decode(&manifestResp); err != nil {
		t.Fatalf("packageManifests: decode response: %v", err)
	}
}

func TestE2E_PackageManifests_NoContent_ForUnknownPackage(t *testing.T) {
	githubMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]githubRelease{})
	}))
	defer githubMock.Close()

	repo := newE2ERepo(t, githubMock, []PackageListEntry{})
	svc := NewWingetSrcService(repo, "api.winget-src")
	handler := newE2EHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/packageManifests/nonexistent/pkg", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for unknown package, got %d", w.Code)
	}
}

func TestE2E_ManifestSearch_EmptyResult(t *testing.T) {
	githubMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]githubRelease{})
	}))
	defer githubMock.Close()

	entry := PackageListEntry{
		Provider:      "github",
		Id:            "owner/myapp",
		Name:          "myapp",
		Publisher:     "owner",
		InstallerType: InstallerTypeZipPortable,
	}
	repo := newE2ERepo(t, githubMock, []PackageListEntry{entry})
	// Pre-populate cache with empty versions
	repo.versionCache.Set(entry.Id, []Version{})

	svc := NewWingetSrcService(repo, "api.winget-src")
	handler := newE2EHandler(svc)

	searchBody, _ := json.Marshal(ManifestSearchRequest{
		Query: Query{KeyWord: "nomatch"},
	})
	req := httptest.NewRequest(http.MethodPost, "/manifestSearch", bytes.NewReader(searchBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestE2E_TokenEnv_UsedForAuth(t *testing.T) {
	var gotAuthHeader string
	githubMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuthHeader = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]githubRelease{})
	}))
	defer githubMock.Close()

	t.Setenv("MY_GITHUB_TOKEN", "test-token-from-env")

	entry := PackageListEntry{
		Provider:      "github",
		Id:            "owner/myapp",
		Name:          "myapp",
		Publisher:     "owner",
		TokenEnv:      "MY_GITHUB_TOKEN",
		InstallerType: InstallerTypeZipPortable,
	}

	_, err := Github{httpClient: githubMock.Client(), baseURL: githubMock.URL}.FetchVersions(context.Background(), entry)
	if err != nil {
		t.Fatalf("FetchVersions: %v", err)
	}

	if gotAuthHeader != "token test-token-from-env" {
		t.Errorf("expected Authorization header 'token test-token-from-env', got '%s'", gotAuthHeader)
	}
}
