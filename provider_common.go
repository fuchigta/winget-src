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

func detectArch(lname string) (string, bool) {
	if strings.Contains(lname, "x86_64") || strings.Contains(lname, "x64") {
		return "x64", true
	} else if strings.Contains(lname, "i386") || strings.Contains(lname, "x86") {
		return "x86", true
	} else if strings.Contains(lname, "arm64") {
		return "arm64", true
	}
	return "", false
}

func collectChecksums(assets []releaseAsset) (map[string]string, error) {
	checksums := map[string]string{}
	for _, asset := range assets {
		if asset.IsChecksum {
			cs, err := fetchChecksums(asset.DownloadUrl)
			if err != nil {
				return nil, err
			}
			for k, v := range cs {
				checksums[k] = v
			}
		}
	}
	return checksums, nil
}

func buildZipPortableVersions(entry PackageListEntry, releases []release) ([]Version, error) {
	versions := []Version{}

	for _, rel := range releases {
		checksums, err := collectChecksums(rel.Assets)
		if err != nil {
			return nil, err
		}

		installers := []Installer{}
		for _, asset := range rel.Assets {
			lname := strings.ToLower(asset.Name)
			if !(strings.Contains(lname, "windows") && strings.HasSuffix(lname, ".zip")) {
				continue
			}

			arch, ok := detectArch(lname)
			if !ok {
				continue
			}

			checksum := checksums[asset.Name]

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

func buildInstallerVersions(entry PackageListEntry, releases []release, installerType string, ext string) ([]Version, error) {
	versions := []Version{}

	for _, rel := range releases {
		checksums, err := collectChecksums(rel.Assets)
		if err != nil {
			return nil, err
		}

		installers := []Installer{}
		for _, asset := range rel.Assets {
			lname := strings.ToLower(asset.Name)
			if !(strings.Contains(lname, "windows") && strings.HasSuffix(lname, ext)) {
				continue
			}

			arch, ok := detectArch(lname)
			if !ok {
				continue
			}

			checksum := checksums[asset.Name]

			installers = append(installers, Installer{
				Architecture:    arch,
				InstallerType:   installerType,
				InstallerUrl:    asset.DownloadUrl,
				InstallerSha256: checksum,
				Scope:           "user",
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
