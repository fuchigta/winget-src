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

// FetchVersions implements PackageProvider.
func (g Github) FetchVersions(ctx context.Context, entry PackageListEntry) ([]Version, error) {
	baseURL := g.baseURL
	if baseURL == "" {
		baseURL = "https://api.github.com"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/repos/%s/releases", baseURL, entry.Id), nil)
	if err != nil {
		return nil, fmt.Errorf("github releases API: %w", err)
	}

	if token := entry.GetToken(); token != "" {
		req.Header.Add("Authorization", fmt.Sprintf("token %s", token))
	}

	res, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github releases API: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		contents, _ := io.ReadAll(io.LimitReader(res.Body, maxResponseBodySize))
		return nil, fmt.Errorf("github releases API status %d: %s", res.StatusCode, contents)
	}

	githubReleases := []githubRelease{}
	if err := json.NewDecoder(res.Body).Decode(&githubReleases); err != nil {
		return nil, fmt.Errorf("github releases API response decode: %w", err)
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

	return dispatchInstallerBuilder(ctx, g.httpClient, entry, releases)
}

var _ PackageProvider = Github{}
