package main

import (
	"context"
	"os"
	"strings"
)

const (
	InstallerTypeZipPortable = "zip-portable"
	InstallerTypeMsi         = "msi"
	InstallerTypeExe         = "exe"

	DefaultPackageLocale  = "en-US"
	DefaultInstallerScope = "user"

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
	InstallerType  string `yaml:"installer_type"`
	Locale         string `yaml:"locale"`
	Scope          string `yaml:"scope"`
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

// GetScope returns the configured installer scope, defaulting to "user".
func (e PackageListEntry) GetScope() string {
	if e.Scope != "" {
		return e.Scope
	}
	return DefaultInstallerScope
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
