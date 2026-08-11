package client

import (
	"errors"
	"fmt"
	"net/http"
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

// IsNotFound reports whether err is an MLflow "resource does not exist" error.
// Resources use it in Read to signal the object is gone so Pulumi can reconcile.
func IsNotFound(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.ErrorCode == "RESOURCE_DOES_NOT_EXIST" || apiErr.StatusCode == http.StatusNotFound
	}
	return false
}
