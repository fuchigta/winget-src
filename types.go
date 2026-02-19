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
}

// PackageIdentifier returns the WinGet-compatible package identifier
// by replacing slashes with dots in the Id field.
// e.g. "owner/repo" → "owner.repo"
func (e PackageListEntry) PackageIdentifier() string {
	return strings.ReplaceAll(e.Id, "/", ".")
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
}
