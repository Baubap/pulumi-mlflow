package auth

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/Baubap/pulumi-mlflow/provider/client"
)

// ExperimentPermission grants a user a permission level on an MLflow experiment.
// Requires the `mlflow.server.auth` app.
type ExperimentPermission struct{}

// ExperimentPermissionArgs are the inputs for an experiment permission grant.
type ExperimentPermissionArgs struct {
	// ExperimentId is the target experiment. Changing it forces replacement.
	ExperimentId string `pulumi:"experimentId" provider:"replaceOnChanges"`
	// Username is the grantee. Changing it forces replacement.
	Username string `pulumi:"username" provider:"replaceOnChanges"`
	// Permission is the granted level: READ, EDIT, MANAGE or NO_PERMISSIONS.
	Permission string `pulumi:"permission"`
}

// ExperimentPermissionState is the recorded state of an experiment permission.
type ExperimentPermissionState struct {
	ExperimentPermissionArgs
}

// Annotate places the resource in the "auth" module and documents it.
func (r *ExperimentPermission) Annotate(a infer.Annotator) {
	a.SetToken("auth", "ExperimentPermission")
	a.Describe(r, experimentPermissionDesc)
}

// Annotate documents the input fields.
func (a *ExperimentPermissionArgs) Annotate(an infer.Annotator) {
	an.Describe(&a.ExperimentId, "The experiment id. Changing this forces replacement.")
	an.Describe(&a.Username, "The user to grant the permission to. Changing this forces replacement.")
	an.Describe(&a.Permission, "The permission level granted: `READ`, `EDIT`, `MANAGE` or "+
		"`NO_PERMISSIONS`. Updatable in place.")
}

type experimentPermissionDTO struct {
	Permission string `json:"permission"`
}

type experimentPermissionResponse struct {
	ExperimentPermission experimentPermissionDTO `json:"experiment_permission"`
}

func fetchExperimentPermission(
	ctx context.Context, api *client.Client, experimentID, username string,
) (*experimentPermissionDTO, error) {
	var resp experimentPermissionResponse
	q := url.Values{"experiment_id": {experimentID}, "username": {username}}
	if err := api.Do(ctx, http.MethodGet, "experiments/permissions/get", q, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.ExperimentPermission, nil
}

// Create grants the permission.
func (ExperimentPermission) Create(
	ctx context.Context, req infer.CreateRequest[ExperimentPermissionArgs],
) (infer.CreateResponse[ExperimentPermissionState], error) {
	in := req.Inputs
	id := in.ExperimentId + "/" + in.Username
	state := ExperimentPermissionState{ExperimentPermissionArgs: in}
	if req.DryRun {
		return infer.CreateResponse[ExperimentPermissionState]{ID: id, Output: state}, nil
	}
	api := infer.GetConfig[client.Config](ctx).Client()
	if err := api.Do(ctx, http.MethodPost, "experiments/permissions/create", nil, map[string]any{
		"experiment_id": in.ExperimentId,
		"username":      in.Username,
		"permission":    in.Permission,
	}, nil); err != nil {
		return infer.CreateResponse[ExperimentPermissionState]{}, authPluginError(err)
	}
	return infer.CreateResponse[ExperimentPermissionState]{ID: id, Output: state}, nil
}

// Read syncs the permission level.
func (ExperimentPermission) Read(
	ctx context.Context, req infer.ReadRequest[ExperimentPermissionArgs, ExperimentPermissionState],
) (infer.ReadResponse[ExperimentPermissionArgs, ExperimentPermissionState], error) {
	api := infer.GetConfig[client.Config](ctx).Client()
	experimentID, username := req.State.ExperimentId, req.State.Username
	if experimentID == "" {
		experimentID, username = splitID(req.ID)
	}
	p, err := fetchExperimentPermission(ctx, api, experimentID, username)
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			return infer.ReadResponse[ExperimentPermissionArgs, ExperimentPermissionState]{}, nil
		}
		return infer.ReadResponse[ExperimentPermissionArgs, ExperimentPermissionState]{}, authPluginError(err)
	}
	state := req.State
	state.ExperimentId = experimentID
	state.Username = username
	state.Permission = p.Permission
	inputs := req.Inputs
	inputs.ExperimentId = experimentID
	inputs.Username = username
	inputs.Permission = p.Permission
	return infer.ReadResponse[ExperimentPermissionArgs, ExperimentPermissionState]{
		ID: experimentID + "/" + username, Inputs: inputs, State: state,
	}, nil
}

// Update changes the permission level in place.
func (ExperimentPermission) Update(
	ctx context.Context, req infer.UpdateRequest[ExperimentPermissionArgs, ExperimentPermissionState],
) (infer.UpdateResponse[ExperimentPermissionState], error) {
	in := req.Inputs
	if req.DryRun {
		return infer.UpdateResponse[ExperimentPermissionState]{
			Output: ExperimentPermissionState{ExperimentPermissionArgs: in},
		}, nil
	}
	api := infer.GetConfig[client.Config](ctx).Client()
	if err := api.Do(ctx, http.MethodPatch, "experiments/permissions/update", nil, map[string]any{
		"experiment_id": in.ExperimentId,
		"username":      in.Username,
		"permission":    in.Permission,
	}, nil); err != nil {
		return infer.UpdateResponse[ExperimentPermissionState]{}, err
	}
	return infer.UpdateResponse[ExperimentPermissionState]{
		Output: ExperimentPermissionState{ExperimentPermissionArgs: in},
	}, nil
}

// Delete revokes the permission.
func (ExperimentPermission) Delete(
	ctx context.Context, req infer.DeleteRequest[ExperimentPermissionState],
) (infer.DeleteResponse, error) {
	api := infer.GetConfig[client.Config](ctx).Client()
	q := url.Values{"experiment_id": {req.State.ExperimentId}, "username": {req.State.Username}}
	if err := api.Do(ctx, http.MethodDelete, "experiments/permissions/delete", q, nil, nil); err != nil &&
		!errors.Is(err, client.ErrNotFound) {
		return infer.DeleteResponse{}, err
	}
	return infer.DeleteResponse{}, nil
}
