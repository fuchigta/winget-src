package main

import (
	"context"
	"fmt"
	"os"
	"strings"
)

const (
	InstallerTypeZipPortable = "zip-portable"
	InstallerTypeMsi         = "msi"
	InstallerTypeExe         = "exe"

	DefaultPackageLocale = "en-US"

	// maxResponseBodySize is the maximum response body size to read (1 MB).
	maxResponseBodySize = 1 << 20
)

type PackageListEntry struct {
	Provider       string `yaml:"provider"`
	Id             string `yaml:"id"`
	Name           string `yaml:"name"`
	Publisher      string `yaml:"publisher"`
	Description    string `yaml:"description"`
	License        string `yaml:"license"`
	ExecutableName string `yaml:"executable_name"`
	Endpoint       string `yaml:"endpoint"`
	ProjectID      uint   `yaml:"project_id"`
	Token          string `yaml:"token"`
	TokenEnv       string `yaml:"token_env"`
	InstallerType   string `yaml:"installer_type"`
	Architecture    string `yaml:"architecture"`
	Locale          string `yaml:"locale"`
	Scope           string `yaml:"scope"`
	UpgradeBehavior string `yaml:"upgrade_behavior"`
	ProductCode     string `yaml:"product_code"`
}

// PackageIdentifier returns the WinGet-compatible package identifier
// by replacing slashes with dots in the Id field.
// e.g. "owner/repo" → "owner.repo"
func (e PackageListEntry) PackageIdentifier() string {
	return strings.ReplaceAll(e.Id, "/", ".")
}

// GetLocale returns the configured locale, defaulting to "en-US".
func (e PackageListEntry) GetLocale() string {
	if e.Locale != "" {
		return e.Locale
	}
	return DefaultPackageLocale
}

// GetScope returns the scope with installer-type-aware defaults.
// MSI defaults to "machine" (typically installs to Program Files).
// Other types return "" when not explicitly set (WinGet accepts any scope).
func (e PackageListEntry) GetScope() string {
	if e.Scope != "" {
		return e.Scope
	}
	if e.InstallerType == InstallerTypeMsi {
		return "machine"
	}
	return ""
}

// GetUpgradeBehavior returns the upgrade behavior, defaulting to "install".
func (e PackageListEntry) GetUpgradeBehavior() string {
	if e.UpgradeBehavior != "" {
		return e.UpgradeBehavior
	}
	return "install"
}

// validateArchitecture returns an error if the given architecture value is not valid.
// Valid values are "", "x64", "x86", and "arm64". An empty string means auto-detect from filename.
func validateArchitecture(arch string) error {
	switch arch {
	case "", "x64", "x86", "arm64":
		return nil
	default:
		return fmt.Errorf("invalid architecture %q: must be one of x64, x86, arm64", arch)
	}
}

// GetToken returns the token for this entry. Token takes precedence over TokenEnv.
func (e PackageListEntry) GetToken() string {
	if e.Token != "" {
		return e.Token
	}
	if e.TokenEnv != "" {
		return os.Getenv(e.TokenEnv)
	}
	return ""
}

type Version struct {
	Version    string
	Installers []Installer
}

type PackageProvider interface {
	FetchVersions(ctx context.Context, entry PackageListEntry) ([]Version, error)
	FetchReleaseNames(ctx context.Context, entry PackageListEntry) ([]string, error)
}
