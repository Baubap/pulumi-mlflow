package registry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/blang/semver"
	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi-go-provider/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Baubap/pulumi-mlflow/provider/client"
)

// tagFakeServer is a minimal MLflow that stores registered-model tags in memory
// and serves them back via registered-models/get, enough for the tag lifecycle.
func tagFakeServer(t *testing.T) integration.Server {
	t.Helper()
	var mu sync.Mutex
	tags := map[string]string{} // key: "<name>|<key>"

	mux := http.NewServeMux()
	base := "/api/2.0/mlflow/"
	mux.HandleFunc("/version", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("2.16.2"))
	})
	mux.HandleFunc(base+"registered-models/set-tag", func(w http.ResponseWriter, r *http.Request) {
		var b struct{ Name, Key, Value string }
		_ = json.NewDecoder(r.Body).Decode(&b)
		mu.Lock()
		tags[b.Name+"|"+b.Key] = b.Value
		mu.Unlock()
		writeJSON(w, map[string]any{})
	})
	mux.HandleFunc(base+"registered-models/delete-tag", func(w http.ResponseWriter, r *http.Request) {
		name, key := r.URL.Query().Get("name"), r.URL.Query().Get("key")
		mu.Lock()
		delete(tags, name+"|"+key)
		mu.Unlock()
		writeJSON(w, map[string]any{})
	})
	mux.HandleFunc(base+"registered-models/get", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		mu.Lock()
		var kvs []client.KeyValue
		for k, v := range tags {
			if strings.HasPrefix(k, name+"|") {
				kvs = append(kvs, client.KeyValue{Key: strings.TrimPrefix(k, name+"|"), Value: v})
			}
		}
		mu.Unlock()
		writeJSON(w, map[string]any{"registered_model": map[string]any{"name": name, "tags": kvs}})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	t.Setenv("MLFLOW_TRACKING_URI", srv.URL)

	prov, err := infer.NewProviderBuilder().
		WithConfig(infer.Config(&client.Config{})).
		WithResources(infer.Resource(RegisteredModelTag{})).
		Build()
	require.NoError(t, err)
	s, err := integration.NewServer(t.Context(), "mlflow", semver.Version{Minor: 1},
		integration.WithProvider(prov))
	require.NoError(t, err)
	require.NoError(t, s.Configure(p.ConfigureRequest{
		Args: propMap(map[string]any{"trackingUri": srv.URL}),
	}))
	return s
}

// TestRegisteredModelTagLifecycle: set a tag on a model owned elsewhere, update
// its value, and delete just that tag.
func TestRegisteredModelTagLifecycle(t *testing.T) {
	server := tagFakeServer(t)
	integration.LifeCycleTest{
		Resource: "mlflow:registry:RegisteredModelTag",
		Create: integration.Operation{
			Inputs: propMap(map[string]any{
				"name":  "fraud-detector",
				"key":   "model_serving_host",
				"value": "https://ml-a-prod.baubap.com",
			}),
			Hook: func(_, output property.Map) {
				assert.Equal(t, "https://ml-a-prod.baubap.com", output.Get("value").AsString())
			},
		},
		Updates: []integration.Operation{{
			Inputs: propMap(map[string]any{
				"name":  "fraud-detector",
				"key":   "model_serving_host",
				"value": "https://ml-b-prod.baubap.com",
			}),
			Hook: func(_, output property.Map) {
				assert.Equal(t, "https://ml-b-prod.baubap.com", output.Get("value").AsString())
			},
		}},
	}.Run(t, server)
}
