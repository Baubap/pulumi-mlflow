package auth

import (
	"context"
	"net/http"
	"net/url"

	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/Baubap/pulumi-mlflow/provider/client"
)

// RegisteredModelPermission grants a user a permission level on an MLflow
// registered model. Requires the `mlflow.server.auth` app.
type RegisteredModelPermission struct{}

// RegisteredModelPermissionArgs are the inputs for a registered-model permission grant.
type RegisteredModelPermissionArgs struct {
	// Name is the target registered model. Changing it forces replacement.
	Name string `pulumi:"name" provider:"replaceOnChanges"`
	// Username is the grantee. Changing it forces replacement.
	Username string `pulumi:"username" provider:"replaceOnChanges"`
	// Permission is the granted level: READ, EDIT, MANAGE or NO_PERMISSIONS.
	Permission string `pulumi:"permission"`
}

// RegisteredModelPermissionState is the recorded state of the grant.
type RegisteredModelPermissionState struct {
	RegisteredModelPermissionArgs
}

// Annotate places the resource in the "auth" module and documents it.
func (r *RegisteredModelPermission) Annotate(a infer.Annotator) {
	a.SetToken("auth", "RegisteredModelPermission")
	a.Describe(r, registeredModelPermissionDesc)
}

// Annotate documents the input fields.
func (a *RegisteredModelPermissionArgs) Annotate(an infer.Annotator) {
	an.Describe(&a.Name, "The registered model name. Changing this forces replacement.")
	an.Describe(&a.Username, "The user to grant the permission to. Changing this forces replacement.")
	an.Describe(&a.Permission, "The permission level granted: `READ`, `EDIT`, `MANAGE` or "+
		"`NO_PERMISSIONS`. Updatable in place.")
}

type registeredModelPermissionDTO struct {
	Permission string `json:"permission"`
}

type registeredModelPermissionResponse struct {
	RegisteredModelPermission registeredModelPermissionDTO `json:"registered_model_permission"`
}

func fetchRegisteredModelPermission(
	ctx context.Context, api *client.Client, name, username string,
) (*registeredModelPermissionDTO, error) {
	var resp registeredModelPermissionResponse
	q := url.Values{"name": {name}, "username": {username}}
	if err := api.Do(ctx, http.MethodGet, "registered-models/permissions/get", q, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.RegisteredModelPermission, nil
}

// Create grants the permission.
func (RegisteredModelPermission) Create(
	ctx context.Context, req infer.CreateRequest[RegisteredModelPermissionArgs],
) (infer.CreateResponse[RegisteredModelPermissionState], error) {
	in := req.Inputs
	id := in.Name + "/" + in.Username
	state := RegisteredModelPermissionState{RegisteredModelPermissionArgs: in}
	if req.DryRun {
		return infer.CreateResponse[RegisteredModelPermissionState]{ID: id, Output: state}, nil
	}
	api := infer.GetConfig[client.Config](ctx).Client()
	if err := api.Do(ctx, http.MethodPost, "registered-models/permissions/create", nil, map[string]any{
		"name":       in.Name,
		"username":   in.Username,
		"permission": in.Permission,
	}, nil); err != nil {
		return infer.CreateResponse[RegisteredModelPermissionState]{}, authPluginError(err)
	}
	return infer.CreateResponse[RegisteredModelPermissionState]{ID: id, Output: state}, nil
}

// Read syncs the permission level.
func (RegisteredModelPermission) Read(
	ctx context.Context, req infer.ReadRequest[RegisteredModelPermissionArgs, RegisteredModelPermissionState],
) (infer.ReadResponse[RegisteredModelPermissionArgs, RegisteredModelPermissionState], error) {
	api := infer.GetConfig[client.Config](ctx).Client()
	name, username := req.State.Name, req.State.Username
	if name == "" {
		name, username = splitID(req.ID)
	}
	p, err := fetchRegisteredModelPermission(ctx, api, name, username)
	if err != nil {
		if client.IsNotFound(err) {
			return infer.ReadResponse[RegisteredModelPermissionArgs, RegisteredModelPermissionState]{}, nil
		}
		return infer.ReadResponse[RegisteredModelPermissionArgs, RegisteredModelPermissionState]{}, authPluginError(err)
	}
	state := req.State
	state.Name = name
	state.Username = username
	state.Permission = p.Permission
	inputs := req.Inputs
	inputs.Name = name
	inputs.Username = username
	inputs.Permission = p.Permission
	return infer.ReadResponse[RegisteredModelPermissionArgs, RegisteredModelPermissionState]{
		ID: name + "/" + username, Inputs: inputs, State: state,
	}, nil
}

// Update changes the permission level in place.
func (RegisteredModelPermission) Update(
	ctx context.Context, req infer.UpdateRequest[RegisteredModelPermissionArgs, RegisteredModelPermissionState],
) (infer.UpdateResponse[RegisteredModelPermissionState], error) {
	in := req.Inputs
	if req.DryRun {
		return infer.UpdateResponse[RegisteredModelPermissionState]{
			Output: RegisteredModelPermissionState{RegisteredModelPermissionArgs: in},
		}, nil
	}
	api := infer.GetConfig[client.Config](ctx).Client()
	if err := api.Do(ctx, http.MethodPatch, "registered-models/permissions/update", nil, map[string]any{
		"name":       in.Name,
		"username":   in.Username,
		"permission": in.Permission,
	}, nil); err != nil {
		return infer.UpdateResponse[RegisteredModelPermissionState]{}, err
	}
	return infer.UpdateResponse[RegisteredModelPermissionState]{
		Output: RegisteredModelPermissionState{RegisteredModelPermissionArgs: in},
	}, nil
}

// Delete revokes the permission.
func (RegisteredModelPermission) Delete(
	ctx context.Context, req infer.DeleteRequest[RegisteredModelPermissionState],
) (infer.DeleteResponse, error) {
	api := infer.GetConfig[client.Config](ctx).Client()
	q := url.Values{"name": {req.State.Name}, "username": {req.State.Username}}
	if err := api.Do(ctx, http.MethodDelete, "registered-models/permissions/delete", q, nil, nil); err != nil &&
		!client.IsNotFound(err) {
		return infer.DeleteResponse{}, err
	}
	return infer.DeleteResponse{}, nil
}
