package main

type PackageListEntry struct {
	Provider      string `yaml:"provider"`
	Id            string `yaml:"id"`
	Name          string `yaml:"name"`
	Publisher     string `yaml:"publisher"`
	Description   string `yaml:"description"`
	Token         string `yaml:"token"`
	InstallerType string `yaml:"installer_type"`
}

type Version struct {
	Version    string
	Installers []Installer
}

type PackageProvider interface {
	FetchVersions(PackageListEntry) ([]Version, error)
}
