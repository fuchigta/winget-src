package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type Github struct {
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
func (g Github) FetchVersions(entry PackageListEntry) ([]Version, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://api.github.com/repos/%s/releases", entry.Id), nil)
	if err != nil {
		return nil, fmt.Errorf("github releases API: %w", err)
	}

	if len(entry.Token) != 0 {
		req.Header.Add("Authorization", fmt.Sprintf("token %s", entry.Token))
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github releases API: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		contents, _ := io.ReadAll(res.Body)
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
				IsChecksum:  strings.Contains(lname, "checksum") && strings.Contains(a.ContentType, "text/plain"),
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

var _ PackageProvider = Github{}
