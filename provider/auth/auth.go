// Package auth implements resources and data sources for the MLflow
// authentication/access-control REST API (users and experiment/registered-model
// permissions). These require the MLflow server to run with the
// `mlflow.server.auth` app enabled. Resources are placed in the provider's
// "auth" module via SetToken in each resource's Annotate.
package auth

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/Baubap/pulumi-mlflow/provider/client"
)

// Resources returns the auth module's Pulumi resources.
func Resources() []infer.InferredResource {
	return []infer.InferredResource{
		infer.Resource(User{}),
		infer.Resource(ExperimentPermission{}),
		infer.Resource(RegisteredModelPermission{}),
	}
}

// Functions returns the auth module's Pulumi functions (data sources).
func Functions() []infer.InferredFunction {
	return []infer.InferredFunction{
		infer.Function(GetUser{}),
		infer.Function(GetExperimentPermission{}),
		infer.Function(GetRegisteredModelPermission{}),
	}
}

// authPluginError annotates "endpoint missing" failures with a hint that the
// MLflow auth app must be enabled, since the auth REST API only exists then.
func authPluginError(err error) error {
	var apiErr *client.APIError
	if errors.As(err, &apiErr) &&
		(apiErr.ErrorCode == "ENDPOINT_NOT_FOUND" || apiErr.StatusCode == http.StatusNotFound) {
		return fmt.Errorf("%w — the MLflow auth REST API is unavailable; run the server with the "+
			"`mlflow.server.auth` app enabled (e.g. `mlflow server --app-name basic-auth`)", err)
	}
	return err
}

// boolPtrEqual reports whether two *bool values are equal (including nil).
func boolPtrEqual(a, b *bool) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// splitID splits a "<first>/<second>" composite resource id.
func splitID(id string) (string, string) {
	if i := strings.IndexByte(id, '/'); i >= 0 {
		return id[:i], id[i+1:]
	}
	return id, ""
}
