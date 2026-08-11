package auth

import (
	"context"
	"fmt"

	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/Baubap/pulumi-mlflow/provider/client"
)

// GetUser looks up an MLflow user by username.
type GetUser struct{}

// GetUserArgs are the inputs for the getUser function.
type GetUserArgs struct {
	// Username to look up.
	Username string `pulumi:"username"`
}

// GetUserResult is the result of the getUser function.
type GetUserResult struct {
	// Username of the user.
	Username string `pulumi:"username"`
	// IsAdmin reports whether the user is an administrator.
	IsAdmin bool `pulumi:"isAdmin"`
	// UserId is the server-assigned numeric user id.
	UserId string `pulumi:"userId"`
}

// Annotate places the function in the "auth" module and documents it.
func (f *GetUser) Annotate(a infer.Annotator) {
	a.SetToken("auth", "getUser")
	a.Describe(f, getUserDesc)
}

// Invoke returns the user's attributes.
func (GetUser) Invoke(
	ctx context.Context, req infer.FunctionRequest[GetUserArgs],
) (infer.FunctionResponse[GetUserResult], error) {
	api := infer.GetConfig[client.Config](ctx).Client()
	u, err := fetchUser(ctx, api, req.Input.Username)
	if err != nil {
		return infer.FunctionResponse[GetUserResult]{}, authPluginError(err)
	}
	return infer.FunctionResponse[GetUserResult]{Output: GetUserResult{
		Username: u.Username,
		IsAdmin:  u.IsAdmin,
		UserId:   fmt.Sprintf("%d", u.ID),
	}}, nil
}

// GetExperimentPermission looks up a user's permission on an experiment.
type GetExperimentPermission struct{}

// GetExperimentPermissionArgs are the inputs for getExperimentPermission.
type GetExperimentPermissionArgs struct {
	// ExperimentId to look up.
	ExperimentId string `pulumi:"experimentId"`
	// Username to look up.
	Username string `pulumi:"username"`
}

// GetExperimentPermissionResult is the result of getExperimentPermission.
type GetExperimentPermissionResult struct {
	// Permission level granted to the user on the experiment.
	Permission string `pulumi:"permission"`
}

// Annotate places the function in the "auth" module and documents it.
func (f *GetExperimentPermission) Annotate(a infer.Annotator) {
	a.SetToken("auth", "getExperimentPermission")
	a.Describe(f, getExperimentPermissionDesc)
}

// Invoke returns the permission level.
func (GetExperimentPermission) Invoke(
	ctx context.Context, req infer.FunctionRequest[GetExperimentPermissionArgs],
) (infer.FunctionResponse[GetExperimentPermissionResult], error) {
	api := infer.GetConfig[client.Config](ctx).Client()
	p, err := fetchExperimentPermission(ctx, api, req.Input.ExperimentId, req.Input.Username)
	if err != nil {
		return infer.FunctionResponse[GetExperimentPermissionResult]{}, authPluginError(err)
	}
	return infer.FunctionResponse[GetExperimentPermissionResult]{
		Output: GetExperimentPermissionResult{Permission: p.Permission},
	}, nil
}

// GetRegisteredModelPermission looks up a user's permission on a registered model.
type GetRegisteredModelPermission struct{}

// GetRegisteredModelPermissionArgs are the inputs for getRegisteredModelPermission.
type GetRegisteredModelPermissionArgs struct {
	// Name of the registered model.
	Name string `pulumi:"name"`
	// Username to look up.
	Username string `pulumi:"username"`
}

// GetRegisteredModelPermissionResult is the result of getRegisteredModelPermission.
type GetRegisteredModelPermissionResult struct {
	// Permission level granted to the user on the registered model.
	Permission string `pulumi:"permission"`
}

// Annotate places the function in the "auth" module and documents it.
func (f *GetRegisteredModelPermission) Annotate(a infer.Annotator) {
	a.SetToken("auth", "getRegisteredModelPermission")
	a.Describe(f, getRegisteredModelPermissionDesc)
}

// Invoke returns the permission level.
func (GetRegisteredModelPermission) Invoke(
	ctx context.Context, req infer.FunctionRequest[GetRegisteredModelPermissionArgs],
) (infer.FunctionResponse[GetRegisteredModelPermissionResult], error) {
	api := infer.GetConfig[client.Config](ctx).Client()
	p, err := fetchRegisteredModelPermission(ctx, api, req.Input.Name, req.Input.Username)
	if err != nil {
		return infer.FunctionResponse[GetRegisteredModelPermissionResult]{}, authPluginError(err)
	}
	return infer.FunctionResponse[GetRegisteredModelPermissionResult]{
		Output: GetRegisteredModelPermissionResult{Permission: p.Permission},
	}, nil
}
