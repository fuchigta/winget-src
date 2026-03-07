package main

type Query struct {
	KeyWord   string
	MatchType string `json:",omitempty"`
}

const (
	PackageMatchFieldPackageName       = "PackageName"
	PackageMatchFieldProductCode       = "ProductCode"
	PackageMatchFieldPackageFamilyName = "PackageFamilyName"
	PackageMatchFieldPackageIdentifier = "PackageIdentifier"
)

type FieldQuery struct {
	PackageMatchField string
	RequestMatch      Query
}

type ManifestSearchRequest struct {
	Query      Query
	Inclusions []FieldQuery
	Filters    []FieldQuery
}

type ManifestVersion struct {
	PackageVersion string
}

type Manifest struct {
	PackageIdentifier string
	PackageName       string
	Publisher         string
	Versions          []ManifestVersion
}

type ManifestSearchResponse []Manifest

type Authentication struct {
	AuthenticationType string
}

type InformationResponse struct {
	SourceIdentifier              string
	ServerSupportedVersions       []string
	Authentication                Authentication
	UnsupportedPackageMatchFields []string `json:",omitempty"`
}

type NestedInstallerFile struct {
	RelativeFilePath string
}

type AppsAndFeaturesEntry struct {
	DisplayName    string `json:"DisplayName,omitempty"`
	Publisher      string `json:"Publisher,omitempty"`
	DisplayVersion string `json:"DisplayVersion,omitempty"`
	ProductCode    string `json:"ProductCode,omitempty"`
	UpgradeCode    string `json:"UpgradeCode,omitempty"`
	InstallerType  string `json:"InstallerType,omitempty"`
}

type Installer struct {
	Architecture           string
	InstallerType          string
	InstallerUrl           string
	InstallerSha256        string                 `json:"InstallerSha256,omitempty"`
	Scope                  string                 `json:"Scope,omitempty"`
	UpgradeBehavior        string                 `json:"UpgradeBehavior,omitempty"`
	AppsAndFeaturesEntries []AppsAndFeaturesEntry `json:"AppsAndFeaturesEntries,omitempty"`
	NestedInstallerType    string                 `json:"NestedInstallerType,omitempty"`
	NestedInstallerFiles   []NestedInstallerFile  `json:"NestedInstallerFiles,omitempty"`
	InstallerSwitches      map[string]string      `json:"InstallerSwitches,omitempty"`
}

type Locale struct {
	PackageLocale    string
	Publisher        string
	PackageName      string
	License          string
	ShortDescription string
}

type PackageManifestsVersion struct {
	PackageVersion string
	DefaultLocale  Locale
	Installers     []Installer
}

type PackageManifests struct {
	PackageIdentifier string
	Versions          []PackageManifestsVersion
}

type PackageManifestsResponse PackageManifests

type DataResponse struct {
	Data any
}

type ErrorResponseEntry struct {
	ErrorCode    int
	ErrorMessage string
}

type ErrorResponse []ErrorResponseEntry
