package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type Gitlab struct {
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
func (g Gitlab) FetchVersions(entry PackageListEntry) ([]Version, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/api/v4/projects/%d/releases", entry.Endpoint, entry.ProjectID), nil)
	if err != nil {
		return nil, fmt.Errorf("gitlab releases API: %w", err)
	}

	if len(entry.Token) != 0 {
		req.Header.Add("PRIVATE-TOKEN", entry.Token)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gitlab releases API: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		contents, _ := io.ReadAll(res.Body)
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
	case "zip-portable":
		return buildZipPortableVersions(entry, releases)
	default:
		return nil, fmt.Errorf("unknown installer type: %s", entry.InstallerType)
	}
}

var _ PackageProvider = Gitlab{}
