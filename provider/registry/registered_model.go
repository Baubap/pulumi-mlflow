package registry

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/Baubap/pulumi-mlflow/provider/client"
)

// RegisteredModel manages a model in the MLflow Model Registry. The registered
// model name is its unique, immutable identifier.
type RegisteredModel struct{}

// RegisteredModelArgs are the inputs for a RegisteredModel.
type RegisteredModelArgs struct {
	// Name is the unique name of the registered model. Changing it forces a
	// replacement (MLflow's rename is not modeled to keep the resource ID stable).
	Name string `pulumi:"name" provider:"replaceOnChanges"`
	// Description is an optional human-readable description.
	Description *string `pulumi:"description,optional"`
	// Tags is an optional set of key/value metadata tags.
	Tags map[string]string `pulumi:"tags,optional"`
}

// RegisteredModelState is the persisted state for a RegisteredModel.
type RegisteredModelState struct {
	RegisteredModelArgs
	// CreationTimestamp is the model creation time (epoch milliseconds).
	CreationTimestamp int `pulumi:"creationTimestamp"`
	// LastUpdatedTimestamp is the last update time (epoch milliseconds).
	LastUpdatedTimestamp int `pulumi:"lastUpdatedTimestamp"`
}

// Annotate places the resource in the "registry" module.
func (r *RegisteredModel) Annotate(a infer.Annotator) {
	a.SetToken("registry", "RegisteredModel")
	a.Describe(r, registeredModelDesc)
}

func registeredModelState(in RegisteredModelArgs, dto registeredModelDTO) RegisteredModelState {
	return RegisteredModelState{
		RegisteredModelArgs:  in,
		CreationTimestamp:    int(dto.CreationTimestamp),
		LastUpdatedTimestamp: int(dto.LastUpdatedTimestamp),
	}
}

// Create registers a new model.
func (RegisteredModel) Create(
	ctx context.Context, req infer.CreateRequest[RegisteredModelArgs],
) (infer.CreateResponse[RegisteredModelState], error) {
	in := req.Inputs
	if req.DryRun {
		return infer.CreateResponse[RegisteredModelState]{
			ID:     in.Name,
			Output: RegisteredModelState{RegisteredModelArgs: in},
		}, nil
	}
	api := infer.GetConfig[client.Config](ctx).Client()
	body := map[string]any{"name": in.Name}
	if in.Description != nil {
		body["description"] = *in.Description
	}
	if len(in.Tags) > 0 {
		body["tags"] = client.TagsToKV(in.Tags)
	}
	var resp struct {
		RegisteredModel registeredModelDTO `json:"registered_model"`
	}
	if err := api.Do(ctx, http.MethodPost, "registered-models/create", nil, body, &resp); err != nil {
		return infer.CreateResponse[RegisteredModelState]{}, err
	}
	return infer.CreateResponse[RegisteredModelState]{
		ID:     in.Name,
		Output: registeredModelState(in, resp.RegisteredModel),
	}, nil
}

// Read refreshes a RegisteredModel from the server.
func (RegisteredModel) Read(
	ctx context.Context, req infer.ReadRequest[RegisteredModelArgs, RegisteredModelState],
) (infer.ReadResponse[RegisteredModelArgs, RegisteredModelState], error) {
	api := infer.GetConfig[client.Config](ctx).Client()
	dto, err := getRegisteredModel(ctx, api, req.ID)
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			return infer.ReadResponse[RegisteredModelArgs, RegisteredModelState]{}, nil
		}
		return infer.ReadResponse[RegisteredModelArgs, RegisteredModelState]{}, err
	}
	in := RegisteredModelArgs{
		Name:        dto.Name,
		Description: strPtrOrNil(dto.Description),
		Tags:        client.KVToMap(dto.Tags),
	}
	return infer.ReadResponse[RegisteredModelArgs, RegisteredModelState]{
		ID:     req.ID,
		Inputs: in,
		State:  registeredModelState(in, *dto),
	}, nil
}

// Update mutates description and tags in place (name is replace-only).
func (RegisteredModel) Update(
	ctx context.Context, req infer.UpdateRequest[RegisteredModelArgs, RegisteredModelState],
) (infer.UpdateResponse[RegisteredModelState], error) {
	in, old := req.Inputs, req.State
	if req.DryRun {
		return infer.UpdateResponse[RegisteredModelState]{
			Output: RegisteredModelState{
				RegisteredModelArgs:  in,
				CreationTimestamp:    old.CreationTimestamp,
				LastUpdatedTimestamp: old.LastUpdatedTimestamp,
			},
		}, nil
	}
	api := infer.GetConfig[client.Config](ctx).Client()
	if !eqStrPtr(in.Description, old.Description) {
		body := map[string]any{"name": in.Name, "description": deref(in.Description)}
		if err := api.Do(ctx, http.MethodPatch, "registered-models/update", nil, body, nil); err != nil {
			return infer.UpdateResponse[RegisteredModelState]{}, err
		}
	}
	upserts, removals := client.DiffTags(old.Tags, in.Tags)
	for k, v := range upserts {
		body := map[string]any{"name": in.Name, "key": k, "value": v}
		if err := api.Do(ctx, http.MethodPost, "registered-models/set-tag", nil, body, nil); err != nil {
			return infer.UpdateResponse[RegisteredModelState]{}, err
		}
	}
	for _, k := range removals {
		body := map[string]any{"name": in.Name, "key": k}
		q := url.Values{"name": {in.Name}, "key": {k}}
		if err := api.Do(ctx, http.MethodDelete, "registered-models/delete-tag", q, body, nil); err != nil {
			return infer.UpdateResponse[RegisteredModelState]{}, err
		}
	}
	dto, err := getRegisteredModel(ctx, api, in.Name)
	if err != nil {
		return infer.UpdateResponse[RegisteredModelState]{}, err
	}
	return infer.UpdateResponse[RegisteredModelState]{Output: registeredModelState(in, *dto)}, nil
}

// Delete removes a registered model.
func (RegisteredModel) Delete(
	ctx context.Context, req infer.DeleteRequest[RegisteredModelState],
) (infer.DeleteResponse, error) {
	api := infer.GetConfig[client.Config](ctx).Client()
	body := map[string]any{"name": req.State.Name}
	q := url.Values{"name": {req.State.Name}}
	err := api.Do(ctx, http.MethodDelete, "registered-models/delete", q, body, nil)
	if err != nil && errors.Is(err, client.ErrNotFound) {
		err = nil
	}
	return infer.DeleteResponse{}, err
}

func getRegisteredModel(ctx context.Context, api *client.Client, name string) (*registeredModelDTO, error) {
	var resp struct {
		RegisteredModel registeredModelDTO `json:"registered_model"`
	}
	if err := api.Do(ctx, http.MethodGet, "registered-models/get", url.Values{"name": {name}}, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.RegisteredModel, nil
}
