package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

type mockService struct {
	infoResp     InformationResponse
	searchResp   ManifestSearchResponse
	manifestResp PackageManifestsResponse
	err          error
}

func (m mockService) Information(ctx context.Context) (InformationResponse, error) {
	return m.infoResp, m.err
}

func (m mockService) ManifestSearch(ctx context.Context, req ManifestSearchRequest) (ManifestSearchResponse, error) {
	return m.searchResp, m.err
}

func (m mockService) PackageManifests(ctx context.Context, identifier string, version string) (PackageManifestsResponse, error) {
	if m.err != nil {
		return PackageManifestsResponse{}, m.err
	}
	return m.manifestResp, nil
}

func TestHandler_Health(t *testing.T) {
	handler := NewWingetSrcHandler(mockService{})
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
}

func TestHandler_Information(t *testing.T) {
	svc := mockService{
		infoResp: InformationResponse{
			SourceIdentifier:        "api.winget-src",
			ServerSupportedVersions: []string{"1.4.0"},
		},
	}
	handler := NewWingetSrcHandler(svc)
	req := httptest.NewRequest(http.MethodGet, "/information", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp DataResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
}

func TestHandler_Information_ServiceError(t *testing.T) {
	svc := mockService{err: fmt.Errorf("service error")}
	handler := NewWingetSrcHandler(svc)
	req := httptest.NewRequest(http.MethodGet, "/information", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", w.Code)
	}
}

func TestHandler_ManifestSearch_Success(t *testing.T) {
	svc := mockService{
		searchResp: ManifestSearchResponse{
			{PackageIdentifier: "owner/foo", PackageName: "foo", Publisher: "owner"},
		},
	}
	handler := NewWingetSrcHandler(svc)

	body, _ := json.Marshal(ManifestSearchRequest{Query: Query{Keyword: "foo"}})
	req := httptest.NewRequest(http.MethodPost, "/manifestSearch", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
}

func TestHandler_ManifestSearch_BadRequest(t *testing.T) {
	svc := mockService{}
	handler := NewWingetSrcHandler(svc)

	req := httptest.NewRequest(http.MethodPost, "/manifestSearch", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestHandler_ManifestSearch_ServiceError(t *testing.T) {
	svc := mockService{err: fmt.Errorf("service error")}
	handler := NewWingetSrcHandler(svc)

	body, _ := json.Marshal(ManifestSearchRequest{Query: Query{Keyword: "foo"}})
	req := httptest.NewRequest(http.MethodPost, "/manifestSearch", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", w.Code)
	}
}

func TestHandler_PackageManifests_Success(t *testing.T) {
	svc := mockService{
		manifestResp: PackageManifestsResponse{
			PackageIdentifier: "owner.foo",
			Versions: []PackageManifestsVersion{
				{PackageVersion: "1.0.0"},
			},
		},
	}
	handler := NewWingetSrcHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/packageManifests/owner.foo", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
}

func TestHandler_PackageManifests_NoContent(t *testing.T) {
	svc := mockService{
		manifestResp: PackageManifestsResponse{},
	}
	handler := NewWingetSrcHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/packageManifests/nonexistent", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", w.Code)
	}
}

func TestHandler_PackageManifests_ServiceError(t *testing.T) {
	svc := mockService{err: fmt.Errorf("service error")}
	handler := NewWingetSrcHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/packageManifests/owner.foo", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", w.Code)
	}
}
