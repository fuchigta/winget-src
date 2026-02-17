package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var httpClient = &http.Client{
	Timeout: 30 * time.Second,
}

type releaseAsset struct {
	Name        string
	DownloadUrl string
	IsChecksum  bool
}

type release struct {
	Name   string
	Assets []releaseAsset
}

func buildZipPortableVersions(entry PackageListEntry, releases []release) ([]Version, error) {
	versions := []Version{}

	for _, rel := range releases {
		installers := []Installer{}
		checksums := map[string]string{}

		for _, asset := range rel.Assets {
			if asset.IsChecksum {
				cs, err := fetchChecksums(asset.DownloadUrl)
				if err != nil {
					return nil, err
				}
				for k, v := range cs {
					checksums[k] = v
				}
			}

			lname := strings.ToLower(asset.Name)
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

			checksum, ok := checksums[asset.Name]
			if !ok {
				checksum = ""
			}

			installers = append(installers, Installer{
				Architecture:        arch,
				InstallerType:       "zip",
				InstallerUrl:        asset.DownloadUrl,
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
			Version:    rel.Name,
			Installers: installers,
		})
	}

	return versions, nil
}

func fetchChecksums(url string) (map[string]string, error) {
	res, err := httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("checksum download: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		contents, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("checksum download status %d: %s", res.StatusCode, contents)
	}

	checksums := map[string]string{}
	scanner := bufio.NewScanner(res.Body)
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

	return checksums, nil
}
