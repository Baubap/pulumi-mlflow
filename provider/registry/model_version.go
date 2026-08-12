package registry

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/Baubap/pulumi-mlflow/provider/client"
)

// ModelVersion manages a version of a registered model in the MLflow Model
// Registry. Its identity is the parent model name plus the server-assigned
// version number.
type ModelVersion struct{}

// ModelVersionArgs are the inputs for a ModelVersion.
type ModelVersionArgs struct {
	// Name is the parent registered model name. Changing it forces a replacement.
	Name string `pulumi:"name" provider:"replaceOnChanges"`
	// Source is the URI of the model artifacts. Changing it forces a replacement.
	Source string `pulumi:"source" provider:"replaceOnChanges"`
	// RunId is the optional MLflow run that produced the model. Changing it forces a replacement.
	RunId *string `pulumi:"runId,optional" provider:"replaceOnChanges"`
	// RunLink is an optional link to the source run. Changing it forces a replacement.
	RunLink *string `pulumi:"runLink,optional" provider:"replaceOnChanges"`
	// Description is an optional human-readable description.
	Description *string `pulumi:"description,optional"`
	// Tags is an optional set of key/value metadata tags.
	Tags map[string]string `pulumi:"tags,optional"`
	// Stage is the (deprecated) model stage: None, Staging, Production or Archived.
	// Stages are deprecated in MLflow 3 in favor of aliases; prefer RegisteredModelAlias.
	Stage *string `pulumi:"stage,optional"`
}

// ModelVersionState is the persisted state for a ModelVersion.
type ModelVersionState struct {
	ModelVersionArgs
	// Version is the server-assigned version number.
	Version string `pulumi:"version"`
	// CurrentStage is the version's current stage.
	CurrentStage string `pulumi:"currentStage"`
	// Status is the registration status (e.g. READY).
	Status string `pulumi:"status"`
	// Aliases lists the aliases currently pointing at this version.
	Aliases []string `pulumi:"aliases,optional"`
	// CreationTimestamp is the creation time (epoch milliseconds).
	CreationTimestamp int `pulumi:"creationTimestamp"`
	// LastUpdatedTimestamp is the last update time (epoch milliseconds).
	LastUpdatedTimestamp int `pulumi:"lastUpdatedTimestamp"`
}

// Annotate places the resource in the "registry" module and deprecates Stage.
func (r *ModelVersion) Annotate(a infer.Annotator) {
	a.SetToken("registry", "ModelVersion")
	a.Describe(r, modelVersionDesc)
}

// Annotate documents ModelVersion inputs and marks Stage deprecated.
func (m *ModelVersionArgs) Annotate(a infer.Annotator) {
	a.Describe(&m.Name, "Parent registered model name. Changing it replaces the version.")
	a.Describe(&m.Source, "URI of the model artifacts (e.g. an s3:// path). Set once; changing it replaces the version.")
	a.Describe(&m.RunId, "Optional MLflow run that produced the model. Changing it replaces the version.")
	a.Describe(&m.RunLink, "Optional link to the source run. Changing it replaces the version.")
	a.Describe(&m.Description, "Optional human-readable description. Updated in place.")
	a.Describe(&m.Tags, "Optional key/value metadata tags. Synced in place.")
	a.Describe(&m.Stage, "Deprecated model stage (None/Staging/Production/Archived). Prefer RegisteredModelAlias; setting this on MLflow 3 emits a warning.")
	a.Deprecate(&m.Stage, "Model stages are deprecated in MLflow 3. Use RegisteredModelAlias instead.")
}

func modelVersionState(in ModelVersionArgs, dto modelVersionDTO) ModelVersionState {
	return ModelVersionState{
		ModelVersionArgs:     in,
		Version:              dto.Version,
		CurrentStage:         dto.CurrentStage,
		Status:               dto.Status,
		Aliases:              dto.Aliases,
		CreationTimestamp:    int(dto.CreationTimestamp),
		LastUpdatedTimestamp: int(dto.LastUpdatedTimestamp),
	}
}

func modelVersionID(name, version string) string { return name + "/" + version }

func parseModelVersionID(id string) (name, version string, ok bool) {
	i := strings.LastIndexByte(id, '/')
	if i <= 0 || i == len(id)-1 {
		return "", "", false
	}
	return id[:i], id[i+1:], true
}

// Create registers a new model version, waits for it to become ready, and
// optionally applies a (deprecated) stage transition.
func (ModelVersion) Create(
	ctx context.Context, req infer.CreateRequest[ModelVersionArgs],
) (infer.CreateResponse[ModelVersionState], error) {
	in := req.Inputs
	if req.DryRun {
		return infer.CreateResponse[ModelVersionState]{
			ID:     modelVersionID(in.Name, "0"),
			Output: ModelVersionState{ModelVersionArgs: in},
		}, nil
	}
	api := infer.GetConfig[client.Config](ctx).Client()
	body := map[string]any{"name": in.Name, "source": in.Source}
	if in.RunId != nil {
		body["run_id"] = *in.RunId
	}
	if in.RunLink != nil {
		body["run_link"] = *in.RunLink
	}
	if in.Description != nil {
		body["description"] = *in.Description
	}
	if len(in.Tags) > 0 {
		body["tags"] = client.TagsToKV(in.Tags)
	}
	var resp struct {
		ModelVersion modelVersionDTO `json:"model_version"`
	}
	if err := api.Do(ctx, http.MethodPost, "model-versions/create", nil, body, &resp); err != nil {
		return infer.CreateResponse[ModelVersionState]{}, err
	}
	version := resp.ModelVersion.Version

	dto, err := waitModelVersionReady(ctx, api, in.Name, version)
	if err != nil {
		return infer.CreateResponse[ModelVersionState]{}, err
	}
	if in.Stage != nil && *in.Stage != "" {
		if api.ServerMajorVersion(ctx) >= 3 {
			p.GetLogger(ctx).Warningf(
				"model version stages are deprecated in MLflow 3; prefer RegisteredModelAlias over stage %q", *in.Stage)
		}
		if err := transitionStage(ctx, api, in.Name, version, *in.Stage); err != nil {
			return infer.CreateResponse[ModelVersionState]{}, err
		}
		dto, err = getModelVersion(ctx, api, in.Name, version)
		if err != nil {
			return infer.CreateResponse[ModelVersionState]{}, err
		}
	}
	return infer.CreateResponse[ModelVersionState]{
		ID:     modelVersionID(in.Name, version),
		Output: modelVersionState(in, *dto),
	}, nil
}

// Read refreshes a ModelVersion from the server.
func (ModelVersion) Read(
	ctx context.Context, req infer.ReadRequest[ModelVersionArgs, ModelVersionState],
) (infer.ReadResponse[ModelVersionArgs, ModelVersionState], error) {
	name, version, ok := parseModelVersionID(req.ID)
	if !ok {
		return infer.ReadResponse[ModelVersionArgs, ModelVersionState]{}, fmt.Errorf("invalid model version id %q, want name/version", req.ID)
	}
	api := infer.GetConfig[client.Config](ctx).Client()
	dto, err := getModelVersion(ctx, api, name, version)
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			return infer.ReadResponse[ModelVersionArgs, ModelVersionState]{}, nil
		}
		return infer.ReadResponse[ModelVersionArgs, ModelVersionState]{}, err
	}
	in := ModelVersionArgs{
		Name:        dto.Name,
		Source:      dto.Source,
		RunId:       strPtrOrNil(dto.RunID),
		RunLink:     strPtrOrNil(dto.RunLink),
		Description: strPtrOrNil(dto.Description),
		Tags:        client.KVToMap(dto.Tags),
		Stage:       strPtrOrNil(dto.CurrentStage),
	}
	return infer.ReadResponse[ModelVersionArgs, ModelVersionState]{
		ID:     req.ID,
		Inputs: in,
		State:  modelVersionState(in, *dto),
	}, nil
}

// Update mutates description, tags and (deprecated) stage in place.
func (ModelVersion) Update(
	ctx context.Context, req infer.UpdateRequest[ModelVersionArgs, ModelVersionState],
) (infer.UpdateResponse[ModelVersionState], error) {
	in, old := req.Inputs, req.State
	if req.DryRun {
		return infer.UpdateResponse[ModelVersionState]{
			Output: ModelVersionState{ModelVersionArgs: in, Version: old.Version},
		}, nil
	}
	api := infer.GetConfig[client.Config](ctx).Client()
	version := old.Version
	if !eqStrPtr(in.Description, old.Description) {
		body := map[string]any{"name": in.Name, "version": version, "description": deref(in.Description)}
		if err := api.Do(ctx, http.MethodPatch, "model-versions/update", nil, body, nil); err != nil {
			return infer.UpdateResponse[ModelVersionState]{}, err
		}
	}
	upserts, removals := client.DiffTags(old.Tags, in.Tags)
	for k, v := range upserts {
		body := map[string]any{"name": in.Name, "version": version, "key": k, "value": v}
		if err := api.Do(ctx, http.MethodPost, "model-versions/set-tag", nil, body, nil); err != nil {
			return infer.UpdateResponse[ModelVersionState]{}, err
		}
	}
	for _, k := range removals {
		body := map[string]any{"name": in.Name, "version": version, "key": k}
		q := url.Values{"name": {in.Name}, "version": {version}, "key": {k}}
		if err := api.Do(ctx, http.MethodDelete, "model-versions/delete-tag", q, body, nil); err != nil {
			return infer.UpdateResponse[ModelVersionState]{}, err
		}
	}
	if !eqStrPtr(in.Stage, old.Stage) && in.Stage != nil && *in.Stage != "" {
		if api.ServerMajorVersion(ctx) >= 3 {
			p.GetLogger(ctx).Warningf(
				"model version stages are deprecated in MLflow 3; prefer RegisteredModelAlias over stage %q", *in.Stage)
		}
		if err := transitionStage(ctx, api, in.Name, version, *in.Stage); err != nil {
			return infer.UpdateResponse[ModelVersionState]{}, err
		}
	}
	dto, err := getModelVersion(ctx, api, in.Name, version)
	if err != nil {
		return infer.UpdateResponse[ModelVersionState]{}, err
	}
	return infer.UpdateResponse[ModelVersionState]{Output: modelVersionState(in, *dto)}, nil
}

// Delete removes a model version.
func (ModelVersion) Delete(
	ctx context.Context, req infer.DeleteRequest[ModelVersionState],
) (infer.DeleteResponse, error) {
	api := infer.GetConfig[client.Config](ctx).Client()
	body := map[string]any{"name": req.State.Name, "version": req.State.Version}
	q := url.Values{"name": {req.State.Name}, "version": {req.State.Version}}
	err := api.Do(ctx, http.MethodDelete, "model-versions/delete", q, body, nil)
	if err != nil && errors.Is(err, client.ErrNotFound) {
		err = nil
	}
	return infer.DeleteResponse{}, err
}

func getModelVersion(ctx context.Context, api *client.Client, name, version string) (*modelVersionDTO, error) {
	q := url.Values{"name": {name}, "version": {version}}
	var resp struct {
		ModelVersion modelVersionDTO `json:"model_version"`
	}
	if err := api.Do(ctx, http.MethodGet, "model-versions/get", q, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.ModelVersion, nil
}

func transitionStage(ctx context.Context, api *client.Client, name, version, stage string) error {
	body := map[string]any{"name": name, "version": version, "stage": stage, "archive_existing_versions": false}
	return api.Do(ctx, http.MethodPost, "model-versions/transition-stage", nil, body, nil)
}

// waitModelVersionReady polls until the version leaves PENDING_REGISTRATION.
func waitModelVersionReady(ctx context.Context, api *client.Client, name, version string) (*modelVersionDTO, error) {
	deadline := time.Now().Add(60 * time.Second)
	for {
		dto, err := getModelVersion(ctx, api, name, version)
		if err != nil {
			return nil, err
		}
		switch dto.Status {
		case "FAILED_REGISTRATION":
			return nil, fmt.Errorf("model version %s/%s failed registration", name, version)
		case "PENDING_REGISTRATION":
			// keep waiting
		default:
			return dto, nil
		}
		if time.Now().After(deadline) {
			return dto, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}
