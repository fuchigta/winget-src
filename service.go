package main

type WingetSrcService interface {
	Information() (InformationResponse, error)
	ManifestSearch(req ManifestSearchRequest) (ManifestSearchResponse, error)
	PackageManifests(identifier string) (PackageManifestsResponse, error)
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
	conditons := []QueryManifestConditon{}

	if req.Query.Keyword != "" {
		conditons = append(conditons, ByName(req.Query.Keyword))
	}

	if len(req.Inclusions) != 0 {
		orConds := []QueryManifestConditon{}

		for _, inclusion := range req.Inclusions {
			switch inclusion.PackageMatchField {
			case PackageMatchFieldPackageIdentifier, PackageMatchFieldProductCode:
				orConds = append(orConds, ById(inclusion.RequestMatch.Keyword))
			case PackageMatchFieldPackageName, PackageMatchFieldPackageFamilyName:
				orConds = append(orConds, ByName(inclusion.RequestMatch.Keyword))
			}
		}

		conditons = append(conditons, Or(orConds...))
	}

	if len(req.Filters) != 0 {
		andConds := []QueryManifestConditon{}

		for _, filter := range req.Filters {
			switch filter.PackageMatchField {
			case PackageMatchFieldPackageIdentifier, PackageMatchFieldProductCode:
				andConds = append(andConds, ById(filter.RequestMatch.Keyword))
			case PackageMatchFieldPackageName, PackageMatchFieldPackageFamilyName:
				andConds = append(andConds, ByName(filter.RequestMatch.Keyword))
			}
		}

		conditons = append(conditons, And(andConds...))
	}

	maniests, err := w.repository.QueryManifest(And(conditons...))
	if err != nil {
		return ManifestSearchResponse{}, err
	}

	return maniests, nil
}

func (w WingetSrcServiceImpl) PackageManifests(identifier string) (PackageManifestsResponse, error) {
	_ = identifier
	return PackageManifestsResponse{}, nil
}
