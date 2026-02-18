package main

import "context"

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
	InstallerType  string `yaml:"installer_type"`
}

type Version struct {
	Version    string
	Installers []Installer
}

type PackageProvider interface {
	FetchVersions(ctx context.Context, entry PackageListEntry) ([]Version, error)
}
