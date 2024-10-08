package main

import (
	"bufio"
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

	releases := []gitlabRelease{}
	if err := json.NewDecoder(res.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("gitlab releases API response decode: %w", err)
	}

	switch entry.InstallerType {
	case "zip-portable":
		return g.handleZipPortable(entry, releases)
	default:
		return nil, fmt.Errorf("unknown installer type: %s", entry.InstallerType)
	}

}

func (g Gitlab) handleZipPortable(entry PackageListEntry, releases []gitlabRelease) ([]Version, error) {
	versions := []Version{}

	for _, release := range releases {
		installers := []Installer{}

		checksums := map[string]string{}

		for _, link := range release.Assets.Links {
			lname := strings.ToLower(link.Name)
			if strings.Contains(lname, "checksum") {
				checkSumRes, err := http.Get(link.Url)
				if err != nil {
					return nil, fmt.Errorf("checksum download: %w", err)
				}
				defer checkSumRes.Body.Close()

				if checkSumRes.StatusCode != 200 {
					contents, _ := io.ReadAll(checkSumRes.Body)
					return nil, fmt.Errorf("checksum download status %d: %s", checkSumRes.StatusCode, contents)
				}

				scanner := bufio.NewScanner(checkSumRes.Body)
				for scanner.Scan() {
					line := scanner.Text()
					if err := scanner.Err(); err != nil {
						return nil, fmt.Errorf("checksum read: %w", err)
					}

					fields := strings.Fields(line)
					if len(fields) != 2 {
						return nil, fmt.Errorf("checksum format error")
					}

					checksums[fields[1]] = fields[0]
				}
			}

			if !(strings.Contains(lname, "windows") && strings.HasSuffix(lname, ".zip")) {
				continue
			}

			var arch string
			if strings.Contains(lname, "x86_64") || strings.Contains(lname, "x64") {
				arch = "x64"
			} else if strings.Contains(lname, "i386") || strings.Contains(lname, "x86") {
				arch = "x86"
			} else if strings.Contains(lname, "arm64") {
				arch = "arm64"
			} else {
				continue
			}

			checksum, ok := checksums[link.Name]
			if !ok {
				checksum = ""
			}

			installers = append(installers, Installer{
				Architecture:        arch,
				InstallerType:       "zip",
				InstallerUrl:        link.Url,
				InstallerSha256:     checksum,
				Scope:               "user",
				NestedInstallerType: "portable",
				NestedInstallerFiles: []NestedInstallerFile{
					{
						RelativeFilePath: fmt.Sprintf("%s.exe", entry.Name),
					},
				},
			})
		}

		if len(installers) == 0 {
			continue
		}

		versions = append(versions, Version{
			Version:    release.Name,
			Installers: installers,
		})
	}

	return versions, nil
}

var _ PackageProvider = Gitlab{}
