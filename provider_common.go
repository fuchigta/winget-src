package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type releaseAsset struct {
	Name        string
	DownloadUrl string
	IsChecksum  bool
}

type release struct {
	Name   string
	Assets []releaseAsset
}

// releaseAdapter abstracts the provider-specific parts of fetching releases.
type releaseAdapter interface {
	// buildRequest creates the HTTP request for fetching releases.
	buildRequest(ctx context.Context, entry PackageListEntry) (*http.Request, error)
	// decodeReleases decodes the response body into the common []release format.
	decodeReleases(body io.Reader) ([]release, error)
	// providerName returns the provider name for error messages.
	providerName() string
}

// fetchAndBuildVersions implements the common fetch-decode-build pipeline.
func fetchAndBuildVersions(ctx context.Context, client *http.Client, adapter releaseAdapter, entry PackageListEntry) ([]Version, error) {
	req, err := adapter.buildRequest(ctx, entry)
	if err != nil {
		return nil, fmt.Errorf("%s releases API: %w", adapter.providerName(), err)
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s releases API: %w", adapter.providerName(), err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		contents, _ := io.ReadAll(io.LimitReader(res.Body, maxResponseBodySize))
		return nil, fmt.Errorf("%s releases API status %d: %s", adapter.providerName(), res.StatusCode, contents)
	}

	releases, err := adapter.decodeReleases(res.Body)
	if err != nil {
		return nil, fmt.Errorf("%s releases API response decode: %w", adapter.providerName(), err)
	}

	return dispatchInstallerBuilder(ctx, client, entry, releases)
}

func detectArch(lname string) (string, bool) {
	if strings.Contains(lname, "x86_64") || strings.Contains(lname, "x64") || strings.Contains(lname, "amd64") {
		return "x64", true
	} else if strings.Contains(lname, "i386") || strings.Contains(lname, "x86") {
		return "x86", true
	} else if strings.Contains(lname, "arm64") || strings.Contains(lname, "aarch64") {
		return "arm64", true
	}
	return "", false
}

// hasArchKeyword はファイル名にアーキテクチャキーワードが含まれるか確認する
func hasArchKeyword(lname string) bool {
	_, ok := detectArch(lname)
	return ok
}

func collectChecksums(ctx context.Context, client *http.Client, assets []releaseAsset) (map[string]string, error) {
	checksums := map[string]string{}
	for _, asset := range assets {
		if asset.IsChecksum {
			cs, err := fetchChecksums(ctx, client, asset.DownloadUrl)
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

func buildVersions(ctx context.Context, client *http.Client, releases []release, buildInstallers func(rel release, checksums map[string]string) []Installer) ([]Version, error) {
	versions := []Version{}
	for _, rel := range releases {
		checksums, err := collectChecksums(ctx, client, rel.Assets)
		if err != nil {
			return nil, err
		}
		installers := buildInstallers(rel, checksums)
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

func buildZipPortableVersions(ctx context.Context, client *http.Client, entry PackageListEntry, releases []release) ([]Version, error) {
	return buildVersions(ctx, client, releases, func(rel release, checksums map[string]string) []Installer {
		installers := []Installer{}
		for _, asset := range rel.Assets {
			lname := strings.ToLower(asset.Name)
			if !((strings.Contains(lname, "windows") || hasArchKeyword(lname)) && strings.HasSuffix(lname, ".zip")) {
				continue
			}
			arch, ok := detectArch(lname)
			if !ok {
				continue
			}
			executableName := entry.ExecutableName
			if executableName == "" {
				executableName = fmt.Sprintf("%s.exe", entry.Name)
			}
			installers = append(installers, Installer{
				Architecture:        arch,
				InstallerType:       "zip",
				InstallerUrl:        asset.DownloadUrl,
				InstallerSha256:     checksums[asset.Name],
				Scope:               entry.GetScope(),
				NestedInstallerType: "portable",
				NestedInstallerFiles: []NestedInstallerFile{
					{RelativeFilePath: executableName},
				},
			})
		}
		return installers
	})
}

func buildInstallerVersions(ctx context.Context, client *http.Client, entry PackageListEntry, releases []release, installerType string, ext string) ([]Version, error) {
	return buildVersions(ctx, client, releases, func(rel release, checksums map[string]string) []Installer {
		installers := []Installer{}
		for _, asset := range rel.Assets {
			lname := strings.ToLower(asset.Name)
			if !((strings.Contains(lname, "windows") || hasArchKeyword(lname)) && strings.HasSuffix(lname, ext)) {
				continue
			}
			arch, ok := detectArch(lname)
			if !ok {
				continue
			}
			installers = append(installers, Installer{
				Architecture:    arch,
				InstallerType:   installerType,
				InstallerUrl:    asset.DownloadUrl,
				InstallerSha256: checksums[asset.Name],
				Scope:           entry.GetScope(),
			})
		}
		return installers
	})
}

// dispatchInstallerBuilder routes to the appropriate version builder based on installer type.
func dispatchInstallerBuilder(ctx context.Context, client *http.Client, entry PackageListEntry, releases []release) ([]Version, error) {
	switch entry.InstallerType {
	case InstallerTypeZipPortable:
		return buildZipPortableVersions(ctx, client, entry, releases)
	case InstallerTypeMsi:
		return buildInstallerVersions(ctx, client, entry, releases, InstallerTypeMsi, ".msi")
	case InstallerTypeExe:
		return buildInstallerVersions(ctx, client, entry, releases, InstallerTypeExe, ".exe")
	default:
		return nil, fmt.Errorf("unknown installer type: %s", entry.InstallerType)
	}
}

func fetchChecksums(ctx context.Context, client *http.Client, url string) (map[string]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("checksum request: %w", err)
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("checksum download: %w", err)
	}
	defer res.Body.Close()

	const maxChecksumSize = 1 << 20 // 1MB
	body := io.LimitReader(res.Body, maxChecksumSize)

	if res.StatusCode != http.StatusOK {
		contents, _ := io.ReadAll(body)
		return nil, fmt.Errorf("checksum download status %d: %s", res.StatusCode, contents)
	}

	checksums := map[string]string{}
	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		line := scanner.Text()

		fields := strings.Fields(line)
		if len(fields) != 2 {
			return nil, fmt.Errorf("checksum format error: %q", line)
		}

		checksums[fields[1]] = fields[0]
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("checksum read: %w", err)
	}

	return checksums, nil
}
