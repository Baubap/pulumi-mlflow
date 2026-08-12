// Package client implements a thin MLflow REST API client shared by all provider
// resources and functions. It handles authentication, request/response encoding,
// MLflow's error envelope, and server-version detection.
package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// apiPrefix is the stable MLflow REST API version prefix. It is "2.0" for both
// MLflow 2.x and 3.x servers for the entities this provider manages.
const apiPrefix = "/api/2.0/mlflow"

// Client is a configured MLflow REST client. It is safe for concurrent use.
type Client struct {
	baseURL    string
	httpClient *http.Client
	username   string
	password   string
	token      string

	versionOnce sync.Once
	version     string
	versionErr  error
}

func newHTTPClient(insecure bool) *http.Client {
	transport := &http.Transport{}
	if insecure {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // opt-in via insecureSkipVerify
	}
	return &http.Client{Timeout: 60 * time.Second, Transport: transport}
}

// NewClient constructs a Client. trackingURI must be a non-empty absolute URL.
func NewClient(trackingURI, username, password, token string, insecure bool) (*Client, error) {
	trackingURI = strings.TrimRight(strings.TrimSpace(trackingURI), "/")
	if trackingURI == "" {
		return nil, fmt.Errorf("mlflow trackingUri is required (set mlflow:trackingUri or MLFLOW_TRACKING_URI)")
	}
	u, err := url.Parse(trackingURI)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("invalid mlflow trackingUri %q: must be an absolute URL", trackingURI)
	}
	return &Client{
		baseURL:    trackingURI,
		httpClient: newHTTPClient(insecure),
		username:   username,
		password:   password,
		token:      token,
	}, nil
}

// BaseURL returns the configured tracking URI (without trailing slash).
func (c *Client) BaseURL() string { return c.baseURL }

// Do performs an MLflow REST API call. path is relative to /api/2.0/mlflow
// (e.g. "experiments/create"). For GET and DELETE, params are sent as a query
// string; for POST and PATCH, body is JSON-encoded. out, when non-nil, receives
// the decoded JSON response.
func (c *Client) Do(ctx context.Context, method, path string, query url.Values, body, out any) error {
	endpoint := c.baseURL + apiPrefix + "/" + strings.TrimLeft(path, "/")
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encoding request body: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, reqBody)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	c.authenticate(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("mlflow request %s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading mlflow response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return parseAPIError(resp.StatusCode, data)
	}
	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("decoding mlflow response: %w", err)
		}
	}
	return nil
}

func (c *Client) authenticate(req *http.Request) {
	switch {
	case c.token != "":
		req.Header.Set("Authorization", "Bearer "+c.token)
	case c.username != "" || c.password != "":
		req.SetBasicAuth(c.username, c.password)
	}
}

func parseAPIError(status int, data []byte) error {
	apiErr := &APIError{StatusCode: status}
	var envelope struct {
		ErrorCode string `json:"error_code"`
		Message   string `json:"message"`
	}
	if err := json.Unmarshal(data, &envelope); err == nil {
		apiErr.ErrorCode = envelope.ErrorCode
		apiErr.Message = envelope.Message
	}
	if apiErr.Message == "" {
		apiErr.Message = strings.TrimSpace(string(data))
	}
	return apiErr
}

// ServerVersion returns the MLflow server version via GET {trackingUri}/version.
// The result is cached. It returns an empty string if the endpoint is unavailable
// (older servers or backends that do not expose it), which callers treat as
// "assume 2.x-compatible".
func (c *Client) ServerVersion(ctx context.Context) (string, error) {
	c.versionOnce.Do(func() {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/version", nil)
		if err != nil {
			c.versionErr = err
			return
		}
		c.authenticate(req)
		resp, err := c.httpClient.Do(req)
		if err != nil {
			c.versionErr = err
			return
		}
		defer func() { _ = resp.Body.Close() }()
		data, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == http.StatusOK {
			c.version = strings.TrimSpace(string(data))
		}
	})
	return c.version, c.versionErr
}

// ServerMajorVersion returns the MLflow server major version (e.g. 2 or 3), or 0
// when the version cannot be determined.
func (c *Client) ServerMajorVersion(ctx context.Context) int {
	v, _ := c.ServerVersion(ctx)
	v = strings.TrimSpace(v)
	if i := strings.IndexByte(v, '.'); i > 0 {
		v = v[:i]
	}
	if v == "" {
		return 0
	}
	n := 0
	for _, r := range v {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}
