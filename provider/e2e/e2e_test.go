//go:build e2e

// Package e2e drives the MLflow provider against one or more REAL MLflow
// tracking servers (not the hermetic httptest fakes used by the per-module unit
// tests). It runs the full resource lifecycle (create → update → delete) through
// the provider's in-process gRPC server, so it needs no generated SDKs or Pulumi
// CLI — only reachable MLflow server(s).
//
// It is parametrizable over any number of servers and versions:
//   - MLFLOW_TRACKING_URIS  comma-separated list of tracking server URIs; every
//     scenario runs against each, as a named subtest.
//   - MLFLOW_TRACKING_URI   single-server fallback when URIS is unset.
//
// Optional auth (applied to all servers): MLFLOW_TRACKING_USERNAME/PASSWORD/TOKEN.
// Tests skip when no URI is configured.
//
// Local: make mlflow-up && make test_e2e && make mlflow-down
// Both majors: make test_matrix   |   Many servers: MLFLOW_TRACKING_URIS=a,b,c make test_e2e
package e2e

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/blang/semver"
	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/integration"
	presource "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mlflow "github.com/Baubap/pulumi-mlflow/provider"
	"github.com/Baubap/pulumi-mlflow/provider/client"
)

// serverURIs returns the configured tracking server URIs (MLFLOW_TRACKING_URIS,
// comma-separated) or the single MLFLOW_TRACKING_URI fallback.
func serverURIs() []string {
	if v := strings.TrimSpace(os.Getenv("MLFLOW_TRACKING_URIS")); v != "" {
		var out []string
		for _, u := range strings.Split(v, ",") {
			if u = strings.TrimSpace(u); u != "" {
				out = append(out, u)
			}
		}
		return out
	}
	if v := strings.TrimSpace(os.Getenv("MLFLOW_TRACKING_URI")); v != "" {
		return []string{v}
	}
	return nil
}

// forEachServer runs body against every configured server as a named subtest,
// logging each server's reported version. Skips when none are configured.
//
// It deliberately never prints the raw tracking URI: this suite can run against
// internal servers (multi-server mode) and its output may land in CI logs, so
// servers are identified by index and an opaque hash, never by hostname.
func forEachServer(t *testing.T, body func(t *testing.T, uri string)) {
	t.Helper()
	uris := serverURIs()
	if len(uris) == 0 {
		t.Skip("no MLflow server configured; set MLFLOW_TRACKING_URI(S) (or `make mlflow-up`)")
	}
	for i, uri := range uris {
		i, uri := i, uri
		t.Run(subtestName(i), func(t *testing.T) {
			if v, err := newE2EClient(t, uri).ServerVersion(context.Background()); err == nil && v != "" {
				t.Logf("MLflow server #%d (%s) reports version %s", i, uriHash(uri), v)
			}
			body(t, uri)
		})
	}
}

func subtestName(i int) string { return fmt.Sprintf("server-%d", i) }

// uriHash is a short, non-reversible identifier for a tracking URI, so
// multi-server runs stay distinguishable in logs without publishing the
// server's address.
func uriHash(uri string) string {
	sum := sha256.Sum256([]byte(uri))
	return hex.EncodeToString(sum[:])[:8]
}

func propMap(m map[string]any) property.Map {
	return presource.FromResourcePropertyMap(presource.NewPropertyMapFromMap(m))
}

// uniq returns a collision-resistant name so repeated/parallel runs against the
// same server don't clash and leftover state can't fail a later run.
func uniq(prefix string) string {
	return fmt.Sprintf("pulumi-e2e-%s-%d", prefix, time.Now().UnixNano())
}

func authEnv() (username, password, token string) {
	return os.Getenv("MLFLOW_TRACKING_USERNAME"),
		os.Getenv("MLFLOW_TRACKING_PASSWORD"),
		os.Getenv("MLFLOW_TRACKING_TOKEN")
}

// newE2EServer builds the full provider and configures it against uri.
func newE2EServer(t *testing.T, uri string) integration.Server {
	t.Helper()
	s, err := integration.NewServer(t.Context(), "mlflow", semver.Version{Minor: 1},
		integration.WithProvider(mlflow.Provider()))
	require.NoError(t, err)

	user, pass, token := authEnv()
	args := map[string]any{"trackingUri": uri}
	if user != "" {
		args["username"] = user
	}
	if pass != "" {
		args["password"] = pass
	}
	if token != "" {
		args["token"] = token
	}
	require.NoError(t, s.Configure(p.ConfigureRequest{Args: propMap(args)}))
	return s
}

// newE2EClient returns a raw MLflow REST client for arranging prerequisites
// (e.g. the parent registered model a ModelVersion needs) and cleanup.
func newE2EClient(t *testing.T, uri string) *client.Client {
	t.Helper()
	user, pass, token := authEnv()
	c, err := client.NewClient(uri, user, pass, token, false)
	require.NoError(t, err)
	return c
}

func createModel(t *testing.T, c *client.Client, name string) {
	t.Helper()
	require.NoError(t, c.Do(context.Background(), "POST", "registered-models/create", nil,
		map[string]any{"name": name}, nil))
	t.Cleanup(func() {
		_ = c.Do(context.Background(), "DELETE", "registered-models/delete",
			url.Values{"name": {name}}, nil, nil)
	})
}

// TestExperimentLifecycle: create → rename + retag → delete, per server.
func TestExperimentLifecycle(t *testing.T) {
	forEachServer(t, func(t *testing.T, uri string) {
		server := newE2EServer(t, uri)
		name := uniq("exp")
		integration.LifeCycleTest{
			Resource: "mlflow:index:Experiment",
			Create: integration.Operation{
				Inputs: propMap(map[string]any{
					"name": name,
					"tags": map[string]any{"team": "foundations"},
				}),
				Hook: func(_, output property.Map) {
					assert.NotEmpty(t, output.Get("experimentId").AsString())
					assert.Equal(t, "active", output.Get("lifecycleStage").AsString())
				},
			},
			Updates: []integration.Operation{{
				Inputs: propMap(map[string]any{
					"name": name + "-renamed",
					"tags": map[string]any{"team": "foundations", "env": "e2e"},
				}),
				Hook: func(_, output property.Map) {
					assert.Equal(t, name+"-renamed", output.Get("name").AsString())
				},
			}},
		}.Run(t, server)
	})
}

// TestRegisteredModelLifecycle: create → update description + tags → delete.
func TestRegisteredModelLifecycle(t *testing.T) {
	forEachServer(t, func(t *testing.T, uri string) {
		server := newE2EServer(t, uri)
		name := uniq("model")
		integration.LifeCycleTest{
			Resource: "mlflow:registry:RegisteredModel",
			Create: integration.Operation{
				Inputs: propMap(map[string]any{
					"name":        name,
					"description": "first",
					"tags":        map[string]any{"owner": "foundations"},
				}),
				Hook: func(_, output property.Map) {
					assert.Equal(t, name, output.Get("name").AsString())
					assert.Equal(t, "first", output.Get("description").AsString())
				},
			},
			Updates: []integration.Operation{{
				Inputs: propMap(map[string]any{
					"name":        name,
					"description": "second",
					"tags":        map[string]any{"owner": "foundations", "env": "e2e"},
				}),
				Hook: func(_, output property.Map) {
					assert.Equal(t, "second", output.Get("description").AsString())
				},
			}},
		}.Run(t, server)
	})
}

// TestModelVersionCreate: arrange a parent model via the client, then create a
// model version through the provider and assert it becomes READY.
func TestModelVersionCreate(t *testing.T) {
	forEachServer(t, func(t *testing.T, uri string) {
		server := newE2EServer(t, uri)
		model := uniq("mv-model")
		createModel(t, newE2EClient(t, uri), model)
		integration.LifeCycleTest{
			Resource: "mlflow:registry:ModelVersion",
			Create: integration.Operation{
				Inputs: propMap(map[string]any{
					"name":        model,
					"source":      "s3://pulumi-e2e/model",
					"description": "e2e version",
				}),
				Hook: func(_, output property.Map) {
					assert.Equal(t, "1", output.Get("version").AsString())
					assert.Equal(t, "READY", output.Get("status").AsString())
				},
			},
		}.Run(t, server)
	})
}

// TestRegisteredModelAlias: arrange model + two versions via the client, then
// manage an alias through the provider (create → repoint version → delete).
func TestRegisteredModelAlias(t *testing.T) {
	forEachServer(t, func(t *testing.T, uri string) {
		server := newE2EServer(t, uri)
		c := newE2EClient(t, uri)
		model := uniq("alias-model")
		createModel(t, c, model)
		for i := 0; i < 2; i++ {
			require.NoError(t, c.Do(context.Background(), "POST", "model-versions/create", nil,
				map[string]any{"name": model, "source": "s3://pulumi-e2e/alias"}, nil))
		}
		integration.LifeCycleTest{
			Resource: "mlflow:registry:RegisteredModelAlias",
			Create: integration.Operation{
				Inputs: propMap(map[string]any{
					"modelName": model, "alias": "champion", "version": "1",
				}),
				Hook: func(_, output property.Map) {
					assert.Equal(t, "1", output.Get("version").AsString())
				},
			},
			Updates: []integration.Operation{{
				Inputs: propMap(map[string]any{
					"modelName": model, "alias": "champion", "version": "2",
				}),
				Hook: func(_, output property.Map) {
					assert.Equal(t, "2", output.Get("version").AsString())
				},
			}},
		}.Run(t, server)
	})
}
