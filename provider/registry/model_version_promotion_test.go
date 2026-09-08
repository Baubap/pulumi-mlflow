package registry

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
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

// promotionFakeServer plays both the source (reads) and destination (writes)
// MLflow servers for the deep-copy orchestration. Artifact bytes are handled by
// the stubbed copyArtifactsFn, so no S3 is touched.
func promotionFakeServer(t *testing.T) integration.Server {
	t.Helper()
	mux := http.NewServeMux()
	base := "/api/2.0/mlflow/"

	mux.HandleFunc("/version", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("2.16.2"))
	})
	// Source model version → gives the artifact source + run id. Version-agnostic
	// so it also satisfies the Read() lookup of the promoted version.
	mux.HandleFunc(base+"model-versions/get", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"model_version": map[string]any{
			"name":    r.URL.Query().Get("name"),
			"version": "3",
			"source":  "s3://test-ml-mlflow-bucket/105/run123/artifacts/model",
			"run_id":  "run123",
		}})
	})
	mux.HandleFunc(base+"runs/get", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{"run": map[string]any{
			"info": map[string]any{
				"run_id":        "run123",
				"experiment_id": "105",
				"artifact_uri":  "s3://test-ml-mlflow-bucket/105/run123/artifacts",
			},
			"data": map[string]any{
				"params":  []client.KeyValue{{Key: "lr", Value: "0.01"}},
				"tags":    []client.KeyValue{{Key: "team", Value: "foundations"}},
				"metrics": []map[string]any{{"key": "auc", "value": 0.9, "timestamp": 1, "step": 0}},
			},
		}})
	})
	mux.HandleFunc(base+"experiments/get", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{"experiment": map[string]any{"name": "fraud-detection"}})
	})
	mux.HandleFunc(base+"experiments/get-by-name", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{"error_code": "RESOURCE_DOES_NOT_EXIST", "message": "no exp"})
	})
	mux.HandleFunc(base+"experiments/create", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{"experiment_id": "200"})
	})
	mux.HandleFunc(base+"runs/create", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{"run": map[string]any{"info": map[string]any{
			"run_id":       "destrun1",
			"artifact_uri": "s3://prod-ml-mlflow-bucket/200/destrun1/artifacts",
		}}})
	})
	mux.HandleFunc(base+"runs/log-batch", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, map[string]any{}) })
	mux.HandleFunc(base+"registered-models/create", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, map[string]any{}) })
	mux.HandleFunc(base+"model-versions/create", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{"model_version": map[string]any{"name": "fraud-detector", "version": "1"}})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	t.Setenv("MLFLOW_TRACKING_URI", srv.URL)

	// Stub the artifact byte-copy (the S3 part) for the hermetic test.
	orig := copyArtifactsFn
	copyArtifactsFn = func(_ context.Context, _, _ *client.Client, _, _ string) error { return nil }
	t.Cleanup(func() { copyArtifactsFn = orig })

	prov, err := infer.NewProviderBuilder().
		WithConfig(infer.Config(&client.Config{})).
		WithResources(infer.Resource(ModelVersionPromotion{})).
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

// TestModelVersionPromotion drives the full cross-server deep-copy orchestration
// (metadata over REST, artifacts stubbed) and asserts the destination outputs.
func TestModelVersionPromotion(t *testing.T) {
	server := promotionFakeServer(t)
	sourceURI := os.Getenv("MLFLOW_TRACKING_URI") // the fake, set by promotionFakeServer
	require.NotEmpty(t, sourceURI)
	integration.LifeCycleTest{
		Resource: "mlflow:registry:ModelVersionPromotion",
		Create: integration.Operation{
			Inputs: propMap(map[string]any{
				"sourceTrackingUri": sourceURI,
				"sourceName":        "fraud-detector",
				"sourceVersion":     "3",
			}),
			Hook: func(_, output property.Map) {
				assert.Equal(t, "1", output.Get("destVersion").AsString())
				assert.Equal(t, "destrun1", output.Get("destRunId").AsString())
				assert.Equal(t, "s3://prod-ml-mlflow-bucket/200/destrun1/artifacts/model", output.Get("destSource").AsString())
			},
		},
	}.Run(t, server)
}
