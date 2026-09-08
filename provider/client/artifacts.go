package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// artifactsPrefix is the MLflow proxied-artifact REST route (available when the
// server runs with --serve-artifacts). It is distinct from the /api/2.0/mlflow
// prefix used by Do, so these calls go through rawRequest.
const artifactsPrefix = "/api/2.0/mlflow-artifacts/artifacts"

// ArtifactFile is one entry returned by the artifact-proxy listing.
type ArtifactFile struct {
	Path     string `json:"path"`
	IsDir    bool   `json:"is_dir"`
	FileSize int64  `json:"file_size"`
}

// rawRequest issues an authenticated request to an absolute path under the base
// URL (bypassing the /api/2.0/mlflow prefix), for the artifact-proxy API.
func (c *Client) rawRequest(
	ctx context.Context, method, fullPath string, query url.Values, body io.Reader,
) (*http.Response, error) {
	endpoint := c.baseURL + fullPath
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, err
	}
	c.authenticate(req)
	return c.httpClient.Do(req)
}

// ListArtifactsProxy lists the immediate children of path via the proxy
// (non-recursive; directories come back with IsDir=true).
func (c *Client) ListArtifactsProxy(ctx context.Context, path string) ([]ArtifactFile, error) {
	q := url.Values{}
	if path != "" {
		q.Set("path", path)
	}
	resp, err := c.rawRequest(ctx, http.MethodGet, artifactsPrefix, q, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, parseAPIError(resp.StatusCode, data)
	}
	var out struct {
		Files []ArtifactFile `json:"files"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("decoding artifact list: %w", err)
	}
	return out.Files, nil
}

// DownloadArtifactProxy fetches one artifact file's bytes via the proxy.
func (c *Client) DownloadArtifactProxy(ctx context.Context, path string) ([]byte, error) {
	resp, err := c.rawRequest(ctx, http.MethodGet, artifactsPrefix+"/"+escapeArtifactPath(path), nil, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, parseAPIError(resp.StatusCode, data)
	}
	return data, nil
}

// UploadArtifactProxy writes bytes to one artifact path via the proxy.
func (c *Client) UploadArtifactProxy(ctx context.Context, path string, data []byte) error {
	resp, err := c.rawRequest(ctx, http.MethodPut, artifactsPrefix+"/"+escapeArtifactPath(path), nil, bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return parseAPIError(resp.StatusCode, b)
	}
	return nil
}

// escapeArtifactPath percent-encodes a path while keeping '/' separators.
func escapeArtifactPath(p string) string {
	return (&url.URL{Path: p}).EscapedPath()
}
