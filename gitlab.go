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
	httpClient *http.Client
	baseURL    string
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
	Name   string       `json:"name"`
	Assets gitlabAssets `json:"assets"`
}

// FetchVersions implements PackageProvider.
func (g Gitlab) FetchVersions(ctx context.Context, entry PackageListEntry) ([]Version, error) {
	baseURL := g.baseURL
	if baseURL == "" {
		baseURL = entry.Endpoint
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/api/v4/projects/%d/releases", baseURL, entry.ProjectID), nil)
	if err != nil {
		return nil, fmt.Errorf("gitlab releases API: %w", err)
	}

	if token := entry.GetToken(); token != "" {
		req.Header.Add("PRIVATE-TOKEN", token)
	}

	res, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gitlab releases API: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		contents, _ := io.ReadAll(io.LimitReader(res.Body, maxResponseBodySize))
		return nil, fmt.Errorf("gitlab releases API status %d: %s", res.StatusCode, contents)
	}

	gitlabReleases := []gitlabRelease{}
	if err := json.NewDecoder(res.Body).Decode(&gitlabReleases); err != nil {
		return nil, fmt.Errorf("gitlab releases API response decode: %w", err)
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
		releases[i] = release{Name: gr.Name, Assets: assets}
	}

	switch entry.InstallerType {
	case InstallerTypeZipPortable:
		return buildZipPortableVersions(ctx, g.httpClient, entry, releases)
	case InstallerTypeMsi:
		return buildInstallerVersions(ctx, g.httpClient, entry, releases, InstallerTypeMsi, ".msi")
	case InstallerTypeExe:
		return buildInstallerVersions(ctx, g.httpClient, entry, releases, InstallerTypeExe, ".exe")
	default:
		return nil, fmt.Errorf("unknown installer type: %s", entry.InstallerType)
	}
}

var _ PackageProvider = Gitlab{}
