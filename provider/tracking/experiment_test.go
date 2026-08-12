package tracking

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi-go-provider/integration"
	presource "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/property"

	"github.com/Baubap/pulumi-mlflow/provider/client"
)

// fakeMLflow is a minimal in-memory MLflow tracking server covering the
// experiment endpoints exercised by the Experiment resource lifecycle.
type fakeMLflow struct {
	mu   sync.Mutex
	name string
	tags map[string]string
	gone bool
}

func (f *fakeMLflow) handler() http.Handler {
	mux := http.NewServeMux()
	const base = "/api/2.0/mlflow/experiments/"

	mux.HandleFunc(base+"create", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name string            `json:"name"`
			Tags []client.KeyValue `json:"tags"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		f.mu.Lock()
		f.name = req.Name
		f.tags = client.KVToMap(req.Tags)
		if f.tags == nil {
			f.tags = map[string]string{}
		}
		f.gone = false
		f.mu.Unlock()
		writeJSON(w, map[string]any{"experiment_id": "1"})
	})

	mux.HandleFunc(base+"get", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.gone {
			w.WriteHeader(http.StatusNotFound)
			writeJSON(w, map[string]any{"error_code": "RESOURCE_DOES_NOT_EXIST", "message": "not found"})
			return
		}
		writeJSON(w, map[string]any{"experiment": map[string]any{
			"experiment_id":     "1",
			"name":              f.name,
			"artifact_location": "s3://bucket/1",
			"lifecycle_stage":   "active",
			"tags":              client.TagsToKV(f.tags),
		}})
	})

	mux.HandleFunc(base+"update", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			NewName string `json:"new_name"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		f.mu.Lock()
		f.name = req.NewName
		f.mu.Unlock()
		writeJSON(w, map[string]any{})
	})

	mux.HandleFunc(base+"set-experiment-tag", func(w http.ResponseWriter, r *http.Request) {
		var req struct{ Key, Value string }
		_ = json.NewDecoder(r.Body).Decode(&req)
		f.mu.Lock()
		f.tags[req.Key] = req.Value
		f.mu.Unlock()
		writeJSON(w, map[string]any{})
	})

	mux.HandleFunc(base+"delete", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.gone = true
		f.mu.Unlock()
		writeJSON(w, map[string]any{})
	})

	return mux
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func newTestProvider() p.Provider {
	prov, err := infer.NewProviderBuilder().
		WithConfig(infer.Config(&client.Config{})).
		WithResources(Resources()...).
		WithFunctions(Functions()...).
		Build()
	if err != nil {
		panic(err)
	}
	return prov
}

func TestExperimentLifecycle(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer((&fakeMLflow{}).handler())
	t.Cleanup(srv.Close)

	server, err := integration.NewServer(t.Context(), "mlflow", semver.Version{Minor: 1},
		integration.WithProvider(newTestProvider()))
	require.NoError(t, err)

	require.NoError(t, server.Configure(p.ConfigureRequest{
		Args: presource.FromResourcePropertyMap(
			presource.NewPropertyMapFromMap(map[string]any{"trackingUri": srv.URL})),
	}))

	integration.LifeCycleTest{
		Resource: "mlflow:index:Experiment",
		Create: integration.Operation{
			Inputs: presource.FromResourcePropertyMap(presource.NewPropertyMapFromMap(map[string]any{
				"name": "exp-a",
				"tags": map[string]any{"team": "ml"},
			})),
			Hook: func(_, output property.Map) {
				assert.Equal(t, "1", output.Get("experimentId").AsString())
				assert.Equal(t, "exp-a", output.Get("name").AsString())
				assert.True(t, strings.HasPrefix(output.Get("artifactUri").AsString(), "s3://"))
			},
		},
		Updates: []integration.Operation{{
			Inputs: presource.FromResourcePropertyMap(presource.NewPropertyMapFromMap(map[string]any{
				"name": "exp-b",
				"tags": map[string]any{"team": "ml", "env": "prod"},
			})),
			Hook: func(_, output property.Map) {
				assert.Equal(t, "exp-b", output.Get("name").AsString())
			},
		}},
	}.Run(t, server)
}
