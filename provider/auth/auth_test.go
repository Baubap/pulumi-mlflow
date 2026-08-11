package auth

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/blang/semver"
	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi-go-provider/integration"
	presource "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Baubap/pulumi-mlflow/provider/client"
)

// fakeAuthServer is an in-memory stand-in for the MLflow auth REST API.
func fakeAuthServer(t *testing.T) *httptest.Server {
	t.Helper()
	var mu sync.Mutex
	admins := map[string]bool{}
	expPerms := map[string]string{}

	decode := func(r *http.Request) map[string]any {
		m := map[string]any{}
		if b, _ := io.ReadAll(r.Body); len(b) > 0 {
			_ = json.Unmarshal(b, &m)
		}
		return m
	}
	writeJSON := func(w http.ResponseWriter, v any) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/2.0/mlflow/users/create", func(w http.ResponseWriter, r *http.Request) {
		body := decode(r)
		mu.Lock()
		admins[body["username"].(string)] = false
		mu.Unlock()
		writeJSON(w, map[string]any{})
	})
	mux.HandleFunc("/api/2.0/mlflow/users/get", func(w http.ResponseWriter, r *http.Request) {
		username := r.URL.Query().Get("username")
		mu.Lock()
		isAdmin := admins[username]
		mu.Unlock()
		writeJSON(w, map[string]any{"user": map[string]any{
			"id": 1, "username": username, "is_admin": isAdmin,
		}})
	})
	mux.HandleFunc("/api/2.0/mlflow/users/update-admin", func(w http.ResponseWriter, r *http.Request) {
		body := decode(r)
		mu.Lock()
		admins[body["username"].(string)], _ = body["is_admin"].(bool)
		mu.Unlock()
		writeJSON(w, map[string]any{})
	})
	mux.HandleFunc("/api/2.0/mlflow/users/update-password", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{})
	})
	mux.HandleFunc("/api/2.0/mlflow/users/delete", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{})
	})

	mux.HandleFunc("/api/2.0/mlflow/experiments/permissions/create", func(w http.ResponseWriter, r *http.Request) {
		body := decode(r)
		mu.Lock()
		expPerms[body["experiment_id"].(string)+"/"+body["username"].(string)], _ = body["permission"].(string)
		mu.Unlock()
		writeJSON(w, map[string]any{})
	})
	mux.HandleFunc("/api/2.0/mlflow/experiments/permissions/get", func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("experiment_id") + "/" + r.URL.Query().Get("username")
		mu.Lock()
		perm := expPerms[key]
		mu.Unlock()
		writeJSON(w, map[string]any{"experiment_permission": map[string]any{"permission": perm}})
	})
	mux.HandleFunc("/api/2.0/mlflow/experiments/permissions/update", func(w http.ResponseWriter, r *http.Request) {
		body := decode(r)
		mu.Lock()
		expPerms[body["experiment_id"].(string)+"/"+body["username"].(string)], _ = body["permission"].(string)
		mu.Unlock()
		writeJSON(w, map[string]any{})
	})
	mux.HandleFunc("/api/2.0/mlflow/experiments/permissions/delete", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func testServer(t *testing.T, srv *httptest.Server) integration.Server {
	t.Helper()
	prov, err := infer.NewProviderBuilder().
		WithConfig(infer.Config(&client.Config{})).
		WithResources(Resources()...).
		WithFunctions(Functions()...).
		Build()
	require.NoError(t, err)
	server, err := integration.NewServer(t.Context(), "mlflow", semver.Version{Minor: 1},
		integration.WithProvider(prov))
	require.NoError(t, err)
	require.NoError(t, server.Configure(p.ConfigureRequest{
		Args: presource.FromResourcePropertyMap(
			presource.NewPropertyMapFromMap(map[string]any{"trackingUri": srv.URL})),
	}))
	return server
}

func inputs(m map[string]any) property.Map {
	return presource.FromResourcePropertyMap(presource.NewPropertyMapFromMap(m))
}

func TestUserLifecycle(t *testing.T) {
	// Note: testServer uses t.Setenv, which is incompatible with t.Parallel.
	server := testServer(t, fakeAuthServer(t))

	integration.LifeCycleTest{
		Resource: "mlflow:auth:User",
		Create: integration.Operation{
			Inputs: inputs(map[string]any{"username": "alice", "password": "s3cret"}),
			Hook: func(_, output property.Map) {
				assert.False(t, output.Get("isAdmin").AsBool())
				assert.Equal(t, "1", output.Get("userId").AsString())
			},
		},
		Updates: []integration.Operation{{
			Inputs: inputs(map[string]any{"username": "alice", "password": "s3cret", "isAdmin": true}),
			Hook: func(_, output property.Map) {
				assert.True(t, output.Get("isAdmin").AsBool())
			},
		}},
	}.Run(t, server)
}

func TestExperimentPermissionLifecycle(t *testing.T) {
	// Note: testServer uses t.Setenv, which is incompatible with t.Parallel.
	server := testServer(t, fakeAuthServer(t))

	integration.LifeCycleTest{
		Resource: "mlflow:auth:ExperimentPermission",
		Create: integration.Operation{
			Inputs: inputs(map[string]any{"experimentId": "42", "username": "alice", "permission": "READ"}),
			Hook: func(_, output property.Map) {
				assert.Equal(t, "READ", output.Get("permission").AsString())
			},
		},
		Updates: []integration.Operation{{
			Inputs: inputs(map[string]any{"experimentId": "42", "username": "alice", "permission": "EDIT"}),
			Hook: func(_, output property.Map) {
				assert.Equal(t, "EDIT", output.Get("permission").AsString())
			},
		}},
	}.Run(t, server)
}
