package main

import "fmt"

type WingetSrcService interface {
	Information() (InformationResponse, error)
	ManifestSearch(req ManifestSearchRequest) (ManifestSearchResponse, error)
	PackageManifests(identifier string, version string) (PackageManifestsResponse, error)
}

type WingetSrcServiceImpl struct {
	repository WingetSrcRepository
}

func NewWingetSrcService(repository WingetSrcRepository) WingetSrcService {
	return WingetSrcServiceImpl{
		repository: repository,
	}
}

func (w WingetSrcServiceImpl) Information() (InformationResponse, error) {
	return InformationResponse{
		SourceIdentifier: "api.winget-src",
		ServerSupportedVersions: []string{
			"1.4.0",
			"1.5.0",
		},
	}, nil
}
func (w WingetSrcServiceImpl) ManifestSearch(req ManifestSearchRequest) (ManifestSearchResponse, error) {
	conditions := []QueryManifestCondition{}

	if req.Query.Keyword != "" {
		conditions = append(conditions, ByName(req.Query.Keyword))
	}

	if len(req.Inclusions) != 0 {
		orConds := []QueryManifestCondition{}

		for _, inclusion := range req.Inclusions {
			switch inclusion.PackageMatchField {
			case PackageMatchFieldPackageIdentifier, PackageMatchFieldProductCode:
				orConds = append(orConds, ById(inclusion.RequestMatch.Keyword))
			case PackageMatchFieldPackageName, PackageMatchFieldPackageFamilyName:
				orConds = append(orConds, ByName(inclusion.RequestMatch.Keyword))
			}
		}

		conditions = append(conditions, Or(orConds...))
	}

	if len(req.Filters) != 0 {
		andConds := []QueryManifestCondition{}

		for _, filter := range req.Filters {
			switch filter.PackageMatchField {
			case PackageMatchFieldPackageIdentifier, PackageMatchFieldProductCode:
				andConds = append(andConds, ById(filter.RequestMatch.Keyword))
			case PackageMatchFieldPackageName, PackageMatchFieldPackageFamilyName:
				andConds = append(andConds, ByName(filter.RequestMatch.Keyword))
			}
		}

		conditions = append(conditions, And(andConds...))
	}

	manifests, err := w.repository.QueryManifest(And(conditions...))
	if err != nil {
		return ManifestSearchResponse{}, err
	}

	return manifests, nil
}

func (w WingetSrcServiceImpl) PackageManifests(identifier string, version string) (PackageManifestsResponse, error) {
	res, err := w.repository.QueryPackageManifests(identifier)
	if err != nil {
		return PackageManifestsResponse{}, err
	}

	if res.PackageIdentifier == "" {
		return PackageManifestsResponse{}, nil
	}

	if len(version) != 0 {
		found := []PackageManifestsVersion{}
		for _, v := range res.Versions {
			if v.PackageVersion == version {
				found = append(found, v)
			}
		}

		if len(found) == 0 {
			return PackageManifestsResponse{}, fmt.Errorf("%s not found", version)
		}

		res.Versions = found
	}

	return PackageManifestsResponse(res), nil
}
