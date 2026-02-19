package main

import (
	"context"
	"fmt"
)

type WingetSrcService interface {
	Information(ctx context.Context) (InformationResponse, error)
	ManifestSearch(ctx context.Context, req ManifestSearchRequest) (ManifestSearchResponse, error)
	PackageManifests(ctx context.Context, identifier string, version string) (PackageManifestsResponse, error)
}

type WingetSrcServiceImpl struct {
	repository       WingetSrcRepository
	sourceIdentifier string
}

func NewWingetSrcService(repository WingetSrcRepository, sourceIdentifier string) WingetSrcService {
	return WingetSrcServiceImpl{
		repository:       repository,
		sourceIdentifier: sourceIdentifier,
	}
}

func (w WingetSrcServiceImpl) Information(ctx context.Context) (InformationResponse, error) {
	return InformationResponse{
		SourceIdentifier: w.sourceIdentifier,
		ServerSupportedVersions: []string{
			"1.4.0",
			"1.5.0",
			"1.9.0",
		},
		Authentication: Authentication{
			AuthenticationType: "none",
		},
	}, nil
}
func (w WingetSrcServiceImpl) ManifestSearch(ctx context.Context, req ManifestSearchRequest) (ManifestSearchResponse, error) {
	conditions := []QueryManifestCondition{}

	if req.Query.KeyWord != "" {
		conditions = append(conditions, ByName(req.Query.KeyWord))
	}

	if len(req.Inclusions) != 0 {
		orConds := []QueryManifestCondition{}

		for _, inclusion := range req.Inclusions {
			switch inclusion.PackageMatchField {
			case PackageMatchFieldPackageIdentifier, PackageMatchFieldProductCode:
				orConds = append(orConds, ById(inclusion.RequestMatch.KeyWord))
			case PackageMatchFieldPackageName, PackageMatchFieldPackageFamilyName:
				orConds = append(orConds, ByName(inclusion.RequestMatch.KeyWord))
			}
		}

		conditions = append(conditions, Or(orConds...))
	}

	if len(req.Filters) != 0 {
		andConds := []QueryManifestCondition{}

		for _, filter := range req.Filters {
			switch filter.PackageMatchField {
			case PackageMatchFieldPackageIdentifier, PackageMatchFieldProductCode:
				andConds = append(andConds, ById(filter.RequestMatch.KeyWord))
			case PackageMatchFieldPackageName, PackageMatchFieldPackageFamilyName:
				andConds = append(andConds, ByName(filter.RequestMatch.KeyWord))
			}
		}

		conditions = append(conditions, And(andConds...))
	}

	manifests, err := w.repository.QueryManifest(ctx, And(conditions...))
	if err != nil {
		return ManifestSearchResponse{}, err
	}

	return manifests, nil
}

func (w WingetSrcServiceImpl) PackageManifests(ctx context.Context, identifier string, version string) (PackageManifestsResponse, error) {
	res, err := w.repository.QueryPackageManifests(ctx, identifier)
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
			return PackageManifestsResponse{}, fmt.Errorf("version %s not found for package %s", version, identifier)
		}

		res.Versions = found
	}

	return PackageManifestsResponse(res), nil
}
