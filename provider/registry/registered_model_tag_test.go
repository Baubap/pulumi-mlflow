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
	presource "github.com/pulumi/pulumi/sdk/v3/go/common/resource"
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
	models := map[string]bool{} // models that have been tagged, i.e. exist

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
		models[b.Name] = true
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
		if !models[name] {
			mu.Unlock()
			writeErr(w, http.StatusNotFound, "RESOURCE_DOES_NOT_EXIST")
			return
		}
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

// readTag refreshes a RegisteredModelTag the way `pulumi refresh` (state set) or
// `pulumi import` (state empty) does.
func readTag(t *testing.T, s integration.Server, id string, state map[string]any) p.ReadResponse {
	t.Helper()
	resp, err := s.Read(p.ReadRequest{
		ID:         id,
		Urn:        presource.NewURN("test", "test", "", "mlflow:registry:RegisteredModelTag", "tag"),
		Properties: propMap(state),
	})
	require.NoError(t, err)
	return resp
}

// TestRegisteredModelTagReadSlashes: MLflow allows '/' in tag keys (and, in 2.x,
// in model names), so the <model>/<key> ID is ambiguous. Refresh must use the
// saved state; import must find whichever split names an existing tag.
func TestRegisteredModelTagReadSlashes(t *testing.T) {
	cases := []struct {
		name, model, key string
	}{
		{"plain", "fraud-detector", "model_serving_host"},
		{"slash in key", "fraud-detector", "serving/host"},
		{"slash in model (MLflow 2.x)", "team/fraud-detector", "owner"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := tagFakeServer(t)
			_, err := s.Create(p.CreateRequest{
				Urn: presource.NewURN("test", "test", "", "mlflow:registry:RegisteredModelTag", "tag"),
				Properties: propMap(map[string]any{
					"name": tc.model, "key": tc.key, "value": "v",
				}),
			})
			require.NoError(t, err)
			id := tc.model + "/" + tc.key

			refreshed := readTag(t, s, id, map[string]any{"name": tc.model, "key": tc.key, "value": "v"})
			assert.Equal(t, id, refreshed.ID, "refresh must not report the tag as deleted")

			imported := readTag(t, s, id, map[string]any{})
			require.Equal(t, id, imported.ID, "import must find the tag")
			assert.Equal(t, tc.model, imported.Properties.Get("name").AsString())
			assert.Equal(t, tc.key, imported.Properties.Get("key").AsString())
		})
	}
}
