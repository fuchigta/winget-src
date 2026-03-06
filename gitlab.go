package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type Gitlab struct {
	httpClient     *http.Client
	downloadClient *http.Client
	sha256Fetcher  *SHA256Fetcher
	baseURL        string
}

type gitlabAssetLink struct {
	Name     string `json:"name"`
	Url      string `json:"url"`
	LinkType string `json:"link_type"`
}

type gitlabAssets struct {
	Links []gitlabAssetLink `json:"links"`
}

type gitlabRelease struct {
	Name    string       `json:"name"`
	TagName string       `json:"tag_name"`
	Assets  gitlabAssets `json:"assets"`
}

func (g Gitlab) providerName() string { return "gitlab" }

func (g Gitlab) buildRequest(ctx context.Context, entry PackageListEntry) (*http.Request, error) {
	baseURL := g.baseURL
	if baseURL == "" {
		baseURL = entry.Endpoint
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/api/v4/projects/%d/releases", baseURL, entry.ProjectID), nil)
	if err != nil {
		return nil, err
	}
	if token := entry.GetToken(); token != "" {
		req.Header.Add("PRIVATE-TOKEN", token)
	}
	return req, nil
}

func (g Gitlab) decodeReleases(body io.Reader) ([]release, error) {
	var gitlabReleases []gitlabRelease
	if err := json.NewDecoder(body).Decode(&gitlabReleases); err != nil {
		return nil, err
	}
	releases := make([]release, len(gitlabReleases))
	for i, gr := range gitlabReleases {
		assets := make([]releaseAsset, len(gr.Assets.Links))
		for j, link := range gr.Assets.Links {
			lname := strings.ToLower(link.Name)
			assets[j] = releaseAsset{
				Name:        link.Name,
				DownloadUrl: link.Url,
				IsChecksum:  strings.Contains(lname, "checksum"),
			}
		}
		releases[i] = release{Name: normalizeVersion(gr.TagName), Assets: assets}
	}
	return releases, nil
}

// FetchVersions implements PackageProvider.
func (g Gitlab) FetchVersions(ctx context.Context, entry PackageListEntry) ([]Version, error) {
	return fetchAndBuildVersions(ctx, g.httpClient, g.downloadClient, g.sha256Fetcher, g, entry, nil)
}

// FetchReleaseNames implements PackageProvider.
func (g Gitlab) FetchReleaseNames(ctx context.Context, entry PackageListEntry) ([]string, error) {
	return fetchReleaseNames(ctx, g.httpClient, g, entry)
}

var _ PackageProvider = Gitlab{}
