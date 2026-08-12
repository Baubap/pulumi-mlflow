package registry

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/Baubap/pulumi-mlflow/provider/client"
)

// RegisteredModelAlias manages a named alias on a registered model that points
// at a specific model version (e.g. "champion" -> version 3). Aliases are the
// recommended alternative to deprecated model stages.
type RegisteredModelAlias struct{}

// RegisteredModelAliasArgs are the inputs for a RegisteredModelAlias.
type RegisteredModelAliasArgs struct {
	// ModelName is the registered model the alias belongs to. Changing it forces a replacement.
	ModelName string `pulumi:"modelName" provider:"replaceOnChanges"`
	// Alias is the alias name. Changing it forces a replacement.
	Alias string `pulumi:"alias" provider:"replaceOnChanges"`
	// Version is the model version the alias points at. Updating it re-points the alias.
	Version string `pulumi:"version"`
}

// RegisteredModelAliasState is the persisted state for a RegisteredModelAlias.
type RegisteredModelAliasState struct {
	RegisteredModelAliasArgs
}

// Annotate places the resource in the "registry" module.
func (r *RegisteredModelAlias) Annotate(a infer.Annotator) {
	a.SetToken("registry", "RegisteredModelAlias")
	a.Describe(r, registeredModelAliasDesc)
}

func aliasID(model, alias string) string { return model + "/" + alias }

func parseAliasID(id string) (model, alias string, ok bool) {
	i := strings.LastIndexByte(id, '/')
	if i <= 0 || i == len(id)-1 {
		return "", "", false
	}
	return id[:i], id[i+1:], true
}

func setAlias(ctx context.Context, api *client.Client, model, alias, version string) error {
	body := map[string]any{"name": model, "alias": alias, "version": version}
	return api.Do(ctx, http.MethodPost, "registered-models/alias", nil, body, nil)
}

// Create sets the alias.
func (RegisteredModelAlias) Create(
	ctx context.Context, req infer.CreateRequest[RegisteredModelAliasArgs],
) (infer.CreateResponse[RegisteredModelAliasState], error) {
	in := req.Inputs
	if req.DryRun {
		return infer.CreateResponse[RegisteredModelAliasState]{
			ID:     aliasID(in.ModelName, in.Alias),
			Output: RegisteredModelAliasState{RegisteredModelAliasArgs: in},
		}, nil
	}
	api := infer.GetConfig[client.Config](ctx).Client()
	if err := setAlias(ctx, api, in.ModelName, in.Alias, in.Version); err != nil {
		return infer.CreateResponse[RegisteredModelAliasState]{}, err
	}
	return infer.CreateResponse[RegisteredModelAliasState]{
		ID:     aliasID(in.ModelName, in.Alias),
		Output: RegisteredModelAliasState{RegisteredModelAliasArgs: in},
	}, nil
}

// Read resolves the alias to its current version.
func (RegisteredModelAlias) Read(
	ctx context.Context, req infer.ReadRequest[RegisteredModelAliasArgs, RegisteredModelAliasState],
) (infer.ReadResponse[RegisteredModelAliasArgs, RegisteredModelAliasState], error) {
	model, alias, ok := parseAliasID(req.ID)
	if !ok {
		return infer.ReadResponse[RegisteredModelAliasArgs, RegisteredModelAliasState]{}, fmt.Errorf("invalid alias id %q, want modelName/alias", req.ID)
	}
	api := infer.GetConfig[client.Config](ctx).Client()
	q := url.Values{"name": {model}, "alias": {alias}}
	var resp struct {
		ModelVersion modelVersionDTO `json:"model_version"`
	}
	if err := api.Do(ctx, http.MethodGet, "registered-models/alias", q, nil, &resp); err != nil {
		if errors.Is(err, client.ErrNotFound) {
			return infer.ReadResponse[RegisteredModelAliasArgs, RegisteredModelAliasState]{}, nil
		}
		return infer.ReadResponse[RegisteredModelAliasArgs, RegisteredModelAliasState]{}, err
	}
	in := RegisteredModelAliasArgs{ModelName: model, Alias: alias, Version: resp.ModelVersion.Version}
	return infer.ReadResponse[RegisteredModelAliasArgs, RegisteredModelAliasState]{
		ID:     req.ID,
		Inputs: in,
		State:  RegisteredModelAliasState{RegisteredModelAliasArgs: in},
	}, nil
}

// Update re-points the alias to a new version.
func (RegisteredModelAlias) Update(
	ctx context.Context, req infer.UpdateRequest[RegisteredModelAliasArgs, RegisteredModelAliasState],
) (infer.UpdateResponse[RegisteredModelAliasState], error) {
	in := req.Inputs
	if req.DryRun {
		return infer.UpdateResponse[RegisteredModelAliasState]{Output: RegisteredModelAliasState{RegisteredModelAliasArgs: in}}, nil
	}
	api := infer.GetConfig[client.Config](ctx).Client()
	if err := setAlias(ctx, api, in.ModelName, in.Alias, in.Version); err != nil {
		return infer.UpdateResponse[RegisteredModelAliasState]{}, err
	}
	return infer.UpdateResponse[RegisteredModelAliasState]{Output: RegisteredModelAliasState{RegisteredModelAliasArgs: in}}, nil
}

// Delete removes the alias.
func (RegisteredModelAlias) Delete(
	ctx context.Context, req infer.DeleteRequest[RegisteredModelAliasState],
) (infer.DeleteResponse, error) {
	api := infer.GetConfig[client.Config](ctx).Client()
	in := req.State.RegisteredModelAliasArgs
	body := map[string]any{"name": in.ModelName, "alias": in.Alias}
	q := url.Values{"name": {in.ModelName}, "alias": {in.Alias}}
	err := api.Do(ctx, http.MethodDelete, "registered-models/alias", q, body, nil)
	if err != nil && errors.Is(err, client.ErrNotFound) {
		err = nil
	}
	return infer.DeleteResponse{}, err
}
