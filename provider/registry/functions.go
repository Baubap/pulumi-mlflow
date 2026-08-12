package registry

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/Baubap/pulumi-mlflow/provider/client"
)

// registeredModelResult is the read-only projection of a registered model.
type registeredModelResult struct {
	Name                 string            `pulumi:"name"`
	Description          string            `pulumi:"description"`
	CreationTimestamp    int               `pulumi:"creationTimestamp"`
	LastUpdatedTimestamp int               `pulumi:"lastUpdatedTimestamp"`
	Tags                 map[string]string `pulumi:"tags"`
}

// modelVersionResult is the read-only projection of a model version.
type modelVersionResult struct {
	Name         string            `pulumi:"name"`
	Version      string            `pulumi:"version"`
	CurrentStage string            `pulumi:"currentStage"`
	Description  string            `pulumi:"description"`
	Source       string            `pulumi:"source"`
	RunId        string            `pulumi:"runId"`
	Status       string            `pulumi:"status"`
	Aliases      []string          `pulumi:"aliases"`
	Tags         map[string]string `pulumi:"tags"`
}

func toRegisteredModelResult(d registeredModelDTO) registeredModelResult {
	return registeredModelResult{
		Name:                 d.Name,
		Description:          d.Description,
		CreationTimestamp:    int(d.CreationTimestamp),
		LastUpdatedTimestamp: int(d.LastUpdatedTimestamp),
		Tags:                 client.KVToMap(d.Tags),
	}
}

func toModelVersionResult(d modelVersionDTO) modelVersionResult {
	return modelVersionResult{
		Name:         d.Name,
		Version:      d.Version,
		CurrentStage: d.CurrentStage,
		Description:  d.Description,
		Source:       d.Source,
		RunId:        d.RunID,
		Status:       d.Status,
		Aliases:      d.Aliases,
		Tags:         client.KVToMap(d.Tags),
	}
}

func api(ctx context.Context) *client.Client {
	return infer.GetConfig[client.Config](ctx).Client()
}

// --- GetRegisteredModel ---

// GetRegisteredModel looks up a registered model by name.
type GetRegisteredModel struct{}

type GetRegisteredModelArgs struct {
	Name string `pulumi:"name"`
}

type GetRegisteredModelResult struct {
	registeredModelResult
}

func (f *GetRegisteredModel) Annotate(a infer.Annotator) {
	a.SetToken("registry", "getRegisteredModel")
	a.Describe(f, getRegisteredModelDesc)
}

func (GetRegisteredModel) Invoke(
	ctx context.Context, req infer.FunctionRequest[GetRegisteredModelArgs],
) (infer.FunctionResponse[GetRegisteredModelResult], error) {
	dto, err := getRegisteredModel(ctx, api(ctx), req.Input.Name)
	if err != nil {
		return infer.FunctionResponse[GetRegisteredModelResult]{}, err
	}
	return infer.FunctionResponse[GetRegisteredModelResult]{
		Output: GetRegisteredModelResult{toRegisteredModelResult(*dto)},
	}, nil
}

// --- SearchRegisteredModels ---

// SearchRegisteredModels searches registered models with an optional filter.
type SearchRegisteredModels struct{}

type SearchRegisteredModelsArgs struct {
	Filter     *string `pulumi:"filter,optional"`
	MaxResults *int    `pulumi:"maxResults,optional"`
}

type SearchRegisteredModelsResult struct {
	RegisteredModels []registeredModelResult `pulumi:"registeredModels"`
}

func (f *SearchRegisteredModels) Annotate(a infer.Annotator) {
	a.SetToken("registry", "searchRegisteredModels")
	a.Describe(f, searchRegisteredModelsDesc)
}

func (SearchRegisteredModels) Invoke(
	ctx context.Context, req infer.FunctionRequest[SearchRegisteredModelsArgs],
) (infer.FunctionResponse[SearchRegisteredModelsResult], error) {
	q := url.Values{}
	if req.Input.Filter != nil {
		q.Set("filter", *req.Input.Filter)
	}
	if req.Input.MaxResults != nil {
		q.Set("max_results", strconv.Itoa(*req.Input.MaxResults))
	}
	var resp struct {
		RegisteredModels []registeredModelDTO `json:"registered_models"`
	}
	if err := api(ctx).Do(ctx, http.MethodGet, "registered-models/search", q, nil, &resp); err != nil {
		return infer.FunctionResponse[SearchRegisteredModelsResult]{}, err
	}
	out := SearchRegisteredModelsResult{}
	for _, d := range resp.RegisteredModels {
		out.RegisteredModels = append(out.RegisteredModels, toRegisteredModelResult(d))
	}
	return infer.FunctionResponse[SearchRegisteredModelsResult]{Output: out}, nil
}

// --- GetLatestVersions ---

// GetLatestVersions returns the latest version per requested stage.
type GetLatestVersions struct{}

type GetLatestVersionsArgs struct {
	Name   string   `pulumi:"name"`
	Stages []string `pulumi:"stages,optional"`
}

type GetLatestVersionsResult struct {
	ModelVersions []modelVersionResult `pulumi:"modelVersions"`
}

func (f *GetLatestVersions) Annotate(a infer.Annotator) {
	a.SetToken("registry", "getLatestVersions")
	a.Describe(f, getLatestVersionsDesc)
}

func (GetLatestVersions) Invoke(
	ctx context.Context, req infer.FunctionRequest[GetLatestVersionsArgs],
) (infer.FunctionResponse[GetLatestVersionsResult], error) {
	body := map[string]any{"name": req.Input.Name}
	if len(req.Input.Stages) > 0 {
		body["stages"] = req.Input.Stages
	}
	var resp struct {
		ModelVersions []modelVersionDTO `json:"model_versions"`
	}
	if err := api(ctx).Do(ctx, http.MethodPost, "registered-models/get-latest-versions", nil, body, &resp); err != nil {
		return infer.FunctionResponse[GetLatestVersionsResult]{}, err
	}
	out := GetLatestVersionsResult{}
	for _, d := range resp.ModelVersions {
		out.ModelVersions = append(out.ModelVersions, toModelVersionResult(d))
	}
	return infer.FunctionResponse[GetLatestVersionsResult]{Output: out}, nil
}

// --- GetModelVersion ---

// GetModelVersion looks up a specific model version.
type GetModelVersion struct{}

type GetModelVersionArgs struct {
	Name    string `pulumi:"name"`
	Version string `pulumi:"version"`
}

type GetModelVersionResult struct {
	modelVersionResult
}

func (f *GetModelVersion) Annotate(a infer.Annotator) {
	a.SetToken("registry", "getModelVersion")
	a.Describe(f, getModelVersionDesc)
}

func (GetModelVersion) Invoke(
	ctx context.Context, req infer.FunctionRequest[GetModelVersionArgs],
) (infer.FunctionResponse[GetModelVersionResult], error) {
	dto, err := getModelVersion(ctx, api(ctx), req.Input.Name, req.Input.Version)
	if err != nil {
		return infer.FunctionResponse[GetModelVersionResult]{}, err
	}
	return infer.FunctionResponse[GetModelVersionResult]{Output: GetModelVersionResult{toModelVersionResult(*dto)}}, nil
}

// --- SearchModelVersions ---

// SearchModelVersions searches model versions with an optional filter.
type SearchModelVersions struct{}

type SearchModelVersionsArgs struct {
	Filter     *string `pulumi:"filter,optional"`
	MaxResults *int    `pulumi:"maxResults,optional"`
}

type SearchModelVersionsResult struct {
	ModelVersions []modelVersionResult `pulumi:"modelVersions"`
}

func (f *SearchModelVersions) Annotate(a infer.Annotator) {
	a.SetToken("registry", "searchModelVersions")
	a.Describe(f, searchModelVersionsDesc)
}

func (SearchModelVersions) Invoke(
	ctx context.Context, req infer.FunctionRequest[SearchModelVersionsArgs],
) (infer.FunctionResponse[SearchModelVersionsResult], error) {
	q := url.Values{}
	if req.Input.Filter != nil {
		q.Set("filter", *req.Input.Filter)
	}
	if req.Input.MaxResults != nil {
		q.Set("max_results", strconv.Itoa(*req.Input.MaxResults))
	}
	var resp struct {
		ModelVersions []modelVersionDTO `json:"model_versions"`
	}
	if err := api(ctx).Do(ctx, http.MethodGet, "model-versions/search", q, nil, &resp); err != nil {
		return infer.FunctionResponse[SearchModelVersionsResult]{}, err
	}
	out := SearchModelVersionsResult{}
	for _, d := range resp.ModelVersions {
		out.ModelVersions = append(out.ModelVersions, toModelVersionResult(d))
	}
	return infer.FunctionResponse[SearchModelVersionsResult]{Output: out}, nil
}

// --- GetModelVersionByAlias ---

// GetModelVersionByAlias resolves an alias to its model version.
type GetModelVersionByAlias struct{}

type GetModelVersionByAliasArgs struct {
	Name  string `pulumi:"name"`
	Alias string `pulumi:"alias"`
}

type GetModelVersionByAliasResult struct {
	modelVersionResult
}

func (f *GetModelVersionByAlias) Annotate(a infer.Annotator) {
	a.SetToken("registry", "getModelVersionByAlias")
	a.Describe(f, getModelVersionByAliasDesc)
}

func (GetModelVersionByAlias) Invoke(
	ctx context.Context, req infer.FunctionRequest[GetModelVersionByAliasArgs],
) (infer.FunctionResponse[GetModelVersionByAliasResult], error) {
	q := url.Values{"name": {req.Input.Name}, "alias": {req.Input.Alias}}
	var resp struct {
		ModelVersion modelVersionDTO `json:"model_version"`
	}
	if err := api(ctx).Do(ctx, http.MethodGet, "registered-models/alias", q, nil, &resp); err != nil {
		return infer.FunctionResponse[GetModelVersionByAliasResult]{}, err
	}
	return infer.FunctionResponse[GetModelVersionByAliasResult]{
		Output: GetModelVersionByAliasResult{toModelVersionResult(resp.ModelVersion)},
	}, nil
}

// --- GetModelVersionDownloadUri ---

// GetModelVersionDownloadUri returns the artifact download URI for a version.
type GetModelVersionDownloadUri struct{}

type GetModelVersionDownloadUriArgs struct {
	Name    string `pulumi:"name"`
	Version string `pulumi:"version"`
}

type GetModelVersionDownloadUriResult struct {
	ArtifactUri string `pulumi:"artifactUri"`
}

func (f *GetModelVersionDownloadUri) Annotate(a infer.Annotator) {
	a.SetToken("registry", "getModelVersionDownloadUri")
	a.Describe(f, getModelVersionDownloadUriDesc)
}

func (GetModelVersionDownloadUri) Invoke(
	ctx context.Context, req infer.FunctionRequest[GetModelVersionDownloadUriArgs],
) (infer.FunctionResponse[GetModelVersionDownloadUriResult], error) {
	q := url.Values{"name": {req.Input.Name}, "version": {req.Input.Version}}
	var resp struct {
		ArtifactURI string `json:"artifact_uri"`
	}
	if err := api(ctx).Do(ctx, http.MethodGet, "model-versions/get-download-uri", q, nil, &resp); err != nil {
		return infer.FunctionResponse[GetModelVersionDownloadUriResult]{}, err
	}
	return infer.FunctionResponse[GetModelVersionDownloadUriResult]{
		Output: GetModelVersionDownloadUriResult{ArtifactUri: resp.ArtifactURI},
	}, nil
}
