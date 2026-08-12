package client

import (
	"errors"
	"fmt"
)

// Sentinel errors for well-known MLflow API conditions. Match them with
// [errors.Is], e.g. `errors.Is(err, client.ErrNotFound)`. The concrete
// [*APIError] returned by the client still carries the status code and message;
// these sentinels let callers classify a failure without depending on the code
// string. See the MLflow REST API error codes:
// https://mlflow.org/docs/latest/api_reference/rest-api.html
var (
	// ErrNotFound is reported when a resource does not exist (MLflow error code
	// RESOURCE_DOES_NOT_EXIST). Read handlers treat it as "the object is gone".
	ErrNotFound = errors.New("mlflow: resource does not exist")
	// ErrEndpointNotFound is reported when the REST endpoint itself is missing
	// (MLflow error code ENDPOINT_NOT_FOUND) — e.g. the auth resources are used
	// against a server without the `mlflow.server.auth` app enabled.
	ErrEndpointNotFound = errors.New("mlflow: endpoint not found")
	// ErrAlreadyExists is reported when a resource already exists (MLflow error
	// code RESOURCE_ALREADY_EXISTS).
	ErrAlreadyExists = errors.New("mlflow: resource already exists")
)

// APIError represents an error returned by the MLflow REST API. MLflow encodes
// failures as a JSON envelope {"error_code": "...", "message": "..."}.
type APIError struct {
	StatusCode int
	ErrorCode  string
	Message    string
}

func (e *APIError) Error() string {
	if e.ErrorCode != "" {
		return fmt.Sprintf("mlflow api error (%d %s): %s", e.StatusCode, e.ErrorCode, e.Message)
	}
	return fmt.Sprintf("mlflow api error (%d): %s", e.StatusCode, e.Message)
}

// Is lets [errors.Is] match an *APIError against the package sentinels by MLflow
// error code. Matching is by error code rather than raw HTTP status so that, for
// example, a 404 ENDPOINT_NOT_FOUND (auth app disabled) is not mistaken for a
// missing resource.
func (e *APIError) Is(target error) bool {
	switch target {
	case ErrNotFound:
		return e.ErrorCode == "RESOURCE_DOES_NOT_EXIST"
	case ErrEndpointNotFound:
		return e.ErrorCode == "ENDPOINT_NOT_FOUND"
	case ErrAlreadyExists:
		return e.ErrorCode == "RESOURCE_ALREADY_EXISTS"
	}
	return false
}
