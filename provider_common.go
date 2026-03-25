package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"slices"
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

// fetchReleases fetches and decodes the release list from the provider API.
func fetchReleases(ctx context.Context, apiClient *http.Client, adapter releaseAdapter, entry PackageListEntry) ([]release, error) {
	req, err := adapter.buildRequest(ctx, entry)
	if err != nil {
		return nil, fmt.Errorf("%s releases API: %w", adapter.providerName(), err)
	}

	res, err := apiClient.Do(req)
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
	return releases, nil
}

// fetchAndBuildVersions implements the common fetch-decode-build pipeline.
// fetcher handles SHA256 computation with caching and background deduplication; nil falls back to direct download via client.
// targetVersions, when non-empty, restricts SHA256 computation to only the specified release names.
func fetchAndBuildVersions(ctx context.Context, client *http.Client, fetcher *SHA256Fetcher, adapter releaseAdapter, entry PackageListEntry, targetVersions []string) ([]Version, error) {
	releases, err := fetchReleases(ctx, client, adapter, entry)
	if err != nil {
		return nil, err
	}
	return dispatchInstallerBuilder(ctx, client, fetcher, entry, releases, targetVersions)
}

// fetchReleaseNames fetches only the release names (version strings) without computing SHA256.
func fetchReleaseNames(ctx context.Context, apiClient *http.Client, adapter releaseAdapter, entry PackageListEntry) ([]string, error) {
	releases, err := fetchReleases(ctx, apiClient, adapter, entry)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(releases))
	for i, rel := range releases {
		names[i] = rel.Name
	}
	return names, nil
}

func normalizeVersion(tag string) string {
	return strings.TrimPrefix(tag, "v")
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

type installerConfig struct {
	ext           string
	installerType string
	zipPortable   bool
}

func buildVersionsForConfig(ctx context.Context, client *http.Client, fetcher *SHA256Fetcher, entry PackageListEntry, releases []release, cfg installerConfig, targetVersions []string) ([]Version, error) {
	versions := []Version{}
	for _, rel := range releases {
		if len(targetVersions) > 0 && !slices.Contains(targetVersions, rel.Name) {
			continue
		}
		checksums, err := collectChecksums(ctx, client, rel.Assets)
		if err != nil {
			return nil, err
		}
		installers := []Installer{}
		for _, asset := range rel.Assets {
			lname := strings.ToLower(asset.Name)
			if !strings.HasSuffix(lname, cfg.ext) {
				continue
			}
			var arch string
			if entry.Architecture != "" {
				arch = entry.Architecture
			} else {
				hasArch := false
				arch, hasArch = detectArch(lname)
				if !hasArch {
					arch = "x64"
				}
			}
			sha256hex := checksums[asset.Name]
			if sha256hex == "" {
				var computed string
				var err error
				if fetcher != nil {
					computed, err = fetcher.Fetch(ctx, asset.DownloadUrl)
				} else {
					computed, err = computeSHA256FromURL(ctx, client, asset.DownloadUrl)
				}
				if err != nil {
					return nil, fmt.Errorf("computing sha256 for %s: %w", asset.Name, err)
				}
				sha256hex = computed
			}
			inst := Installer{
				Architecture:    arch,
				InstallerType:   cfg.installerType,
				InstallerUrl:    asset.DownloadUrl,
				InstallerSha256: sha256hex,
				Scope:           entry.GetScope(),
				UpgradeBehavior: entry.GetUpgradeBehavior(),
			}
			if entry.ProductCode != "" {
				inst.AppsAndFeaturesEntries = []AppsAndFeaturesEntry{
					{
						ProductCode:  entry.ProductCode,
						InstallerType: cfg.installerType,
					},
				}
			}
			if cfg.zipPortable {
				executableName := entry.ExecutableName
				if executableName == "" {
					executableName = fmt.Sprintf("%s.exe", entry.Name)
				}
				inst.NestedInstallerType = "portable"
				inst.NestedInstallerFiles = []NestedInstallerFile{
					{RelativeFilePath: executableName},
				}
			}
			installers = append(installers, inst)
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

// getInstallerConfig returns the installerConfig for the given installer type.
func getInstallerConfig(installerType string) (installerConfig, error) {
	switch installerType {
	case InstallerTypeZipPortable:
		return installerConfig{ext: ".zip", installerType: "zip", zipPortable: true}, nil
	case InstallerTypeMsi:
		return installerConfig{ext: ".msi", installerType: InstallerTypeMsi}, nil
	case InstallerTypeExe:
		return installerConfig{ext: ".exe", installerType: InstallerTypeExe}, nil
	default:
		return installerConfig{}, fmt.Errorf("unknown installer type: %s", installerType)
	}
}

// dispatchInstallerBuilder routes to the appropriate version builder based on installer type.
func dispatchInstallerBuilder(ctx context.Context, client *http.Client, fetcher *SHA256Fetcher, entry PackageListEntry, releases []release, targetVersions []string) ([]Version, error) {
	cfg, err := getInstallerConfig(entry.InstallerType)
	if err != nil {
		return nil, err
	}
	return buildVersionsForConfig(ctx, client, fetcher, entry, releases, cfg, targetVersions)
}

// countMatchingReleases counts releases that have at least one asset matching
// the installer config's extension and architecture requirements.
// Uses the same asset matching logic as buildVersionsForConfig but without SHA256 computation.
func countMatchingReleases(releases []release, cfg installerConfig) int {
	count := 0
	for _, rel := range releases {
		for _, asset := range rel.Assets {
			lname := strings.ToLower(asset.Name)
			if strings.HasSuffix(lname, cfg.ext) {
				count++
				break
			}
		}
	}
	return count
}

// computeSHA256FromURL downloads the content at url and returns its SHA-256 hex digest.
// Used as fallback when no checksum file is provided in the release assets.
func computeSHA256FromURL(ctx context.Context, client *http.Client, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("sha256 request: %w", err)
	}
	res, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("sha256 download: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, maxResponseBodySize))
		return "", fmt.Errorf("sha256 download status %d: %s", res.StatusCode, body)
	}
	h := sha256.New()
	if _, err := io.Copy(h, res.Body); err != nil {
		return "", fmt.Errorf("sha256 read: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
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
