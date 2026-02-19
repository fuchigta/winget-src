package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type Github struct {
	httpClient *http.Client
	baseURL    string
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadUrl string `json:"browser_download_url"`
	ContentType        string `json:"content_type"`
}

type githubRelease struct {
	Name   string        `json:"name"`
	Assets []githubAsset `json:"assets"`
}

func (g Github) providerName() string { return "github" }

func (g Github) buildRequest(ctx context.Context, entry PackageListEntry) (*http.Request, error) {
	baseURL := g.baseURL
	if baseURL == "" {
		baseURL = "https://api.github.com"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/repos/%s/releases", baseURL, entry.Id), nil)
	if err != nil {
		return nil, err
	}
	if token := entry.GetToken(); token != "" {
		req.Header.Add("Authorization", fmt.Sprintf("token %s", token))
	}
	return req, nil
}

func (g Github) decodeReleases(body io.Reader) ([]release, error) {
	var githubReleases []githubRelease
	if err := json.NewDecoder(body).Decode(&githubReleases); err != nil {
		return nil, err
	}
	releases := make([]release, len(githubReleases))
	for i, gr := range githubReleases {
		assets := make([]releaseAsset, len(gr.Assets))
		for j, a := range gr.Assets {
			lname := strings.ToLower(a.Name)
			assets[j] = releaseAsset{
				Name:        a.Name,
				DownloadUrl: a.BrowserDownloadUrl,
				// GitHub はファイル名と content-type の両方を確認して false positive を低減する。
				// GitLab のアセットリンクには content-type 情報がないためファイル名のみで判定する。
				IsChecksum: strings.Contains(lname, "checksum") && strings.Contains(a.ContentType, "text/plain"),
			}
		}
		releases[i] = release{Name: gr.Name, Assets: assets}
	}
	return releases, nil
}

// FetchVersions implements PackageProvider.
func (g Github) FetchVersions(ctx context.Context, entry PackageListEntry) ([]Version, error) {
	return fetchAndBuildVersions(ctx, g.httpClient, g, entry)
}

var _ PackageProvider = Github{}
