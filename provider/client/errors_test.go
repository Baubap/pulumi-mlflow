package client

import (
	"errors"
	"fmt"
	"testing"
)

// TestAPIErrorIs verifies the sentinel classification via errors.Is, including
// that an ENDPOINT_NOT_FOUND (e.g. the auth app is disabled) is NOT mistaken for
// a missing resource, and that matching works through a wrapped error chain.
func TestAPIErrorIs(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		target error
		want   bool
	}{
		{"resource-not-found matches ErrNotFound",
			&APIError{StatusCode: 404, ErrorCode: "RESOURCE_DOES_NOT_EXIST"}, ErrNotFound, true},
		{"endpoint-not-found does NOT match ErrNotFound",
			&APIError{StatusCode: 404, ErrorCode: "ENDPOINT_NOT_FOUND"}, ErrNotFound, false},
		{"endpoint-not-found matches ErrEndpointNotFound",
			&APIError{StatusCode: 404, ErrorCode: "ENDPOINT_NOT_FOUND"}, ErrEndpointNotFound, true},
		{"already-exists matches ErrAlreadyExists",
			&APIError{StatusCode: 400, ErrorCode: "RESOURCE_ALREADY_EXISTS"}, ErrAlreadyExists, true},
		{"bare 404 without an error code matches nothing",
			&APIError{StatusCode: 404}, ErrNotFound, false},
		{"wrapped error still matches through %w",
			fmt.Errorf("reading model: %w", &APIError{ErrorCode: "RESOURCE_DOES_NOT_EXIST"}), ErrNotFound, true},
		{"unrelated error code does not match",
			&APIError{ErrorCode: "INVALID_PARAMETER_VALUE"}, ErrNotFound, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := errors.Is(tt.err, tt.target); got != tt.want {
				t.Errorf("errors.Is(%v, %v) = %v, want %v", tt.err, tt.target, got, tt.want)
			}
		})
	}
}
