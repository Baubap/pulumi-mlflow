package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/Baubap/pulumi-mlflow/provider/client"
)

// User manages a user in the MLflow authentication database. It requires the
// MLflow tracking server to run with the `mlflow.server.auth` app enabled.
type User struct{}

// UserArgs are the inputs for an MLflow User.
type UserArgs struct {
	// Username is the unique login name. Changing it forces a new user.
	Username string `pulumi:"username" provider:"replaceOnChanges"`
	// Password is the user's password. It is write-only and never read back.
	Password string `pulumi:"password" provider:"secret"`
	// IsAdmin controls whether the user is an administrator.
	IsAdmin *bool `pulumi:"isAdmin,optional"`
}

// UserState is the recorded state of an MLflow User.
type UserState struct {
	UserArgs
	// UserId is the server-assigned numeric user id.
	UserId string `pulumi:"userId"`
}

// Annotate places the resource in the "auth" module and documents it.
func (u *User) Annotate(a infer.Annotator) {
	a.SetToken("auth", "User")
	a.Describe(u, userDesc)
}

// Annotate documents the input fields.
func (a *UserArgs) Annotate(an infer.Annotator) {
	an.Describe(&a.Username, "The unique username. Changing this forces replacement.")
	an.Describe(&a.Password, "The user's password. Write-only; never read back from the server.")
	an.Describe(&a.IsAdmin, "Whether the user is an administrator (defaults to the server default of false).")
}

// Annotate documents the output-only fields.
func (s *UserState) Annotate(an infer.Annotator) {
	an.Describe(&s.UserId, "The server-assigned numeric user id.")
}

type userDTO struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	IsAdmin  bool   `json:"is_admin"`
}

type userResponse struct {
	User userDTO `json:"user"`
}

func fetchUser(ctx context.Context, api *client.Client, username string) (*userDTO, error) {
	var resp userResponse
	q := url.Values{"username": {username}}
	if err := api.Do(ctx, http.MethodGet, "users/get", q, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.User, nil
}

// Create registers a new MLflow user.
func (User) Create(
	ctx context.Context, req infer.CreateRequest[UserArgs],
) (infer.CreateResponse[UserState], error) {
	in := req.Inputs
	state := UserState{UserArgs: in}
	if req.DryRun {
		return infer.CreateResponse[UserState]{ID: in.Username, Output: state}, nil
	}
	api := infer.GetConfig[client.Config](ctx).Client()
	if err := api.Do(ctx, http.MethodPost, "users/create", nil, map[string]any{
		"username": in.Username,
		"password": in.Password,
	}, nil); err != nil {
		return infer.CreateResponse[UserState]{}, authPluginError(err)
	}
	if in.IsAdmin != nil && *in.IsAdmin {
		if err := api.Do(ctx, http.MethodPatch, "users/update-admin", nil, map[string]any{
			"username": in.Username,
			"is_admin": true,
		}, nil); err != nil {
			return infer.CreateResponse[UserState]{}, err
		}
	}
	if u, err := fetchUser(ctx, api, in.Username); err == nil {
		state.UserId = fmt.Sprintf("%d", u.ID)
		admin := u.IsAdmin
		state.IsAdmin = &admin
	}
	return infer.CreateResponse[UserState]{ID: in.Username, Output: state}, nil
}

// Read syncs state with the server. The password is never read back.
func (User) Read(
	ctx context.Context, req infer.ReadRequest[UserArgs, UserState],
) (infer.ReadResponse[UserArgs, UserState], error) {
	api := infer.GetConfig[client.Config](ctx).Client()
	username := req.ID
	if username == "" {
		username = req.State.Username
	}
	u, err := fetchUser(ctx, api, username)
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			return infer.ReadResponse[UserArgs, UserState]{}, nil
		}
		return infer.ReadResponse[UserArgs, UserState]{}, authPluginError(err)
	}
	state := req.State
	state.Username = u.Username
	admin := u.IsAdmin
	state.IsAdmin = &admin
	state.UserId = fmt.Sprintf("%d", u.ID)
	inputs := req.Inputs
	inputs.Username = u.Username
	return infer.ReadResponse[UserArgs, UserState]{ID: username, Inputs: inputs, State: state}, nil
}

// Update applies password and admin-flag changes in place.
func (User) Update(
	ctx context.Context, req infer.UpdateRequest[UserArgs, UserState],
) (infer.UpdateResponse[UserState], error) {
	in := req.Inputs
	state := req.State
	if req.DryRun {
		state.UserArgs = in
		return infer.UpdateResponse[UserState]{Output: state}, nil
	}
	api := infer.GetConfig[client.Config](ctx).Client()
	if in.Password != req.State.Password {
		if err := api.Do(ctx, http.MethodPatch, "users/update-password", nil, map[string]any{
			"username": in.Username,
			"password": in.Password,
		}, nil); err != nil {
			return infer.UpdateResponse[UserState]{}, err
		}
	}
	if in.IsAdmin != nil && !boolPtrEqual(in.IsAdmin, req.State.IsAdmin) {
		if err := api.Do(ctx, http.MethodPatch, "users/update-admin", nil, map[string]any{
			"username": in.Username,
			"is_admin": *in.IsAdmin,
		}, nil); err != nil {
			return infer.UpdateResponse[UserState]{}, err
		}
	}
	state.UserArgs = in
	if u, err := fetchUser(ctx, api, in.Username); err == nil {
		state.UserId = fmt.Sprintf("%d", u.ID)
		admin := u.IsAdmin
		state.IsAdmin = &admin
	}
	return infer.UpdateResponse[UserState]{Output: state}, nil
}

// Delete removes the user.
func (User) Delete(
	ctx context.Context, req infer.DeleteRequest[UserState],
) (infer.DeleteResponse, error) {
	api := infer.GetConfig[client.Config](ctx).Client()
	q := url.Values{"username": {req.State.Username}}
	if err := api.Do(ctx, http.MethodDelete, "users/delete", q, nil, nil); err != nil && !errors.Is(err, client.ErrNotFound) {
		return infer.DeleteResponse{}, err
	}
	return infer.DeleteResponse{}, nil
}
