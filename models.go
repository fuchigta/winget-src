package main

type Query struct {
	Keyword string
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

type InformationResponse struct {
	SourceIdentifier        string
	ServerSupportedVersions []string
}

type NestedInstallerFile struct {
	RelativeFilePath string
}

const (
	InstallerSwitchesSilent             = "Silent"
	InstallerSwitchesSilentWithProgress = "SilentWithProgress"
	InstallerSwitchesInteractive        = "Interactive"
	InstallerSwitchesInstallLocation    = "InstallLocation"
	InstallerSwitchesLog                = "Log"
	InstallerSwitchesUpgrade            = "Upgrade"
	InstallerSwitchesCustom             = "Custom"
)

type Installer struct {
	Architecture         string
	InstallerType        string
	InstallerUrl         string
	InstallerSha256      string                `json:"InstallerSha256,omitempty"`
	Scope                string                `json:"Scope,omitempty"`
	NestedInstallerType  string                `json:"NestedInstallerType,omitempty"`
	NestedInstallerFiles []NestedInstallerFile `json:"NestedInstallerFiles,omitempty"`
	InstallerSwitches    map[string]string     `json:"InstallerSwitches,omitempty"`
}

type Locale struct {
	PackageLocale    string
	Publisher        string
	PackageName      string
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
	Data interface{}
}

type ErrorReponseEntry struct {
	ErrorCode    int
	ErrorMessage string
}

type ErrorResponse []ErrorReponseEntry
