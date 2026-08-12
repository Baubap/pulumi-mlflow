package registry

import (
	"encoding/json"
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

// fakeMLflow is a minimal in-memory MLflow REST server for hermetic tests.
type fakeMLflow struct {
	mu     sync.Mutex
	models map[string]*registeredModelDTO
	mvs    map[string]*modelVersionDTO // key: name/version
}

func newFakeMLflow() *fakeMLflow {
	return &fakeMLflow{models: map[string]*registeredModelDTO{}, mvs: map[string]*modelVersionDTO{}}
}

func params(r *http.Request) map[string]any {
	out := map[string]any{}
	for k, v := range r.URL.Query() {
		if len(v) > 0 {
			out[k] = v[0]
		}
	}
	if r.Body != nil {
		var body map[string]any
		if json.NewDecoder(r.Body).Decode(&body) == nil {
			for k, v := range body {
				out[k] = v
			}
		}
	}
	return out
}

func (f *fakeMLflow) handler() http.Handler {
	mux := http.NewServeMux()
	base := "/api/2.0/mlflow/"

	mux.HandleFunc("/version", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("2.16.2"))
	})

	mux.HandleFunc(base+"registered-models/create", func(w http.ResponseWriter, r *http.Request) {
		pr := params(r)
		f.mu.Lock()
		defer f.mu.Unlock()
		name, _ := pr["name"].(string)
		m := &registeredModelDTO{Name: name, CreationTimestamp: 1, LastUpdatedTimestamp: 1}
		if d, ok := pr["description"].(string); ok {
			m.Description = d
		}
		f.models[name] = m
		writeJSON(w, map[string]any{"registered_model": m})
	})
	mux.HandleFunc(base+"registered-models/get", func(w http.ResponseWriter, r *http.Request) {
		pr := params(r)
		f.mu.Lock()
		defer f.mu.Unlock()
		name, _ := pr["name"].(string)
		m, ok := f.models[name]
		if !ok {
			writeErr(w, http.StatusNotFound, "RESOURCE_DOES_NOT_EXIST")
			return
		}
		writeJSON(w, map[string]any{"registered_model": m})
	})
	mux.HandleFunc(base+"registered-models/update", func(w http.ResponseWriter, r *http.Request) {
		pr := params(r)
		f.mu.Lock()
		defer f.mu.Unlock()
		name, _ := pr["name"].(string)
		if m, ok := f.models[name]; ok {
			if d, ok := pr["description"].(string); ok {
				m.Description = d
			}
			writeJSON(w, map[string]any{"registered_model": m})
			return
		}
		writeErr(w, http.StatusNotFound, "RESOURCE_DOES_NOT_EXIST")
	})
	mux.HandleFunc(base+"registered-models/set-tag", func(w http.ResponseWriter, r *http.Request) {
		pr := params(r)
		f.mu.Lock()
		defer f.mu.Unlock()
		name, _ := pr["name"].(string)
		key, _ := pr["key"].(string)
		val, _ := pr["value"].(string)
		if m, ok := f.models[name]; ok {
			m.Tags = upsertTag(m.Tags, key, val)
		}
		writeJSON(w, map[string]any{})
	})
	mux.HandleFunc(base+"registered-models/delete-tag", func(w http.ResponseWriter, r *http.Request) {
		pr := params(r)
		f.mu.Lock()
		defer f.mu.Unlock()
		name, _ := pr["name"].(string)
		key, _ := pr["key"].(string)
		if m, ok := f.models[name]; ok {
			m.Tags = removeTag(m.Tags, key)
		}
		writeJSON(w, map[string]any{})
	})
	mux.HandleFunc(base+"registered-models/delete", func(w http.ResponseWriter, r *http.Request) {
		pr := params(r)
		f.mu.Lock()
		defer f.mu.Unlock()
		delete(f.models, pr["name"].(string))
		writeJSON(w, map[string]any{})
	})

	mux.HandleFunc(base+"model-versions/create", func(w http.ResponseWriter, r *http.Request) {
		pr := params(r)
		f.mu.Lock()
		defer f.mu.Unlock()
		name, _ := pr["name"].(string)
		source, _ := pr["source"].(string)
		mv := &modelVersionDTO{Name: name, Version: "1", Source: source, Status: "READY", CreationTimestamp: 1, LastUpdatedTimestamp: 1}
		f.mvs[name+"/1"] = mv
		writeJSON(w, map[string]any{"model_version": mv})
	})
	mux.HandleFunc(base+"model-versions/get", func(w http.ResponseWriter, r *http.Request) {
		pr := params(r)
		f.mu.Lock()
		defer f.mu.Unlock()
		name, _ := pr["name"].(string)
		version, _ := pr["version"].(string)
		mv, ok := f.mvs[name+"/"+version]
		if !ok {
			writeErr(w, http.StatusNotFound, "RESOURCE_DOES_NOT_EXIST")
			return
		}
		writeJSON(w, map[string]any{"model_version": mv})
	})
	mux.HandleFunc(base+"model-versions/delete", func(w http.ResponseWriter, r *http.Request) {
		pr := params(r)
		f.mu.Lock()
		defer f.mu.Unlock()
		name, _ := pr["name"].(string)
		version, _ := pr["version"].(string)
		delete(f.mvs, name+"/"+version)
		writeJSON(w, map[string]any{})
	})

	return mux
}

func upsertTag(tags []client.KeyValue, k, v string) []client.KeyValue {
	for i := range tags {
		if tags[i].Key == k {
			tags[i].Value = v
			return tags
		}
	}
	return append(tags, client.KeyValue{Key: k, Value: v})
}

func removeTag(tags []client.KeyValue, k string) []client.KeyValue {
	out := tags[:0]
	for _, t := range tags {
		if t.Key != k {
			out = append(out, t)
		}
	}
	return out
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, code string) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error_code": code, "message": code})
}

func testServer(t *testing.T) integration.Server {
	t.Helper()
	fake := httptest.NewServer(newFakeMLflow().handler())
	t.Cleanup(fake.Close)
	t.Setenv("MLFLOW_TRACKING_URI", fake.URL)

	prov, err := infer.NewProviderBuilder().
		WithConfig(infer.Config(&client.Config{})).
		WithResources(Resources()...).
		WithFunctions(Functions()...).
		Build()
	require.NoError(t, err)

	s, err := integration.NewServer(t.Context(), "mlflow", semver.Version{Minor: 1},
		integration.WithProvider(prov))
	require.NoError(t, err)

	require.NoError(t, s.Configure(p.ConfigureRequest{
		Args: propMap(map[string]any{"trackingUri": fake.URL}),
	}))
	return s
}

func propMap(m map[string]any) property.Map {
	return presource.FromResourcePropertyMap(presource.NewPropertyMapFromMap(m))
}

func TestRegisteredModelLifecycle(t *testing.T) {
	server := testServer(t)

	integration.LifeCycleTest{
		Resource: "mlflow:registry:RegisteredModel",
		Create: integration.Operation{
			Inputs: propMap(map[string]any{
				"name":        "my-model",
				"description": "first",
				"tags":        map[string]any{"team": "ml"},
			}),
			Hook: func(_, output property.Map) {
				assert.Equal(t, "my-model", output.Get("name").AsString())
				assert.Equal(t, "first", output.Get("description").AsString())
			},
		},
		Updates: []integration.Operation{{
			Inputs: propMap(map[string]any{
				"name":        "my-model",
				"description": "second",
				"tags":        map[string]any{"team": "ml", "env": "prod"},
			}),
			Hook: func(_, output property.Map) {
				assert.Equal(t, "second", output.Get("description").AsString())
			},
		}},
	}.Run(t, server)
}

func TestModelVersionCreate(t *testing.T) {
	server := testServer(t)

	integration.LifeCycleTest{
		Resource: "mlflow:registry:ModelVersion",
		Create: integration.Operation{
			Inputs: propMap(map[string]any{
				"name":   "my-model",
				"source": "s3://bucket/model",
			}),
			Hook: func(_, output property.Map) {
				assert.Equal(t, "1", output.Get("version").AsString())
				assert.Equal(t, "READY", output.Get("status").AsString())
			},
		},
	}.Run(t, server)
}
