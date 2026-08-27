//go:build e2e

package e2e

import (
	"context"
	"net/url"
	"os"
	"strings"
	"testing"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/pulumi/pulumi-go-provider/integration"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestModelVersionPromotionE2E promotes a model version — with real artifacts —
// from a source MLflow server to a destination server, copying the bytes through
// the s3:// backend into a shared MinIO store. See docker-compose.promotion.yml /
// `make promo-up test_e2e_promotion`. Skips when the pair isn't configured.
func TestModelVersionPromotionE2E(t *testing.T) {
	sourceURI := strings.TrimSpace(os.Getenv("MLFLOW_SOURCE_URI"))
	destURI := strings.TrimSpace(os.Getenv("MLFLOW_DEST_URI"))
	if sourceURI == "" || destURI == "" {
		t.Skip("set MLFLOW_SOURCE_URI and MLFLOW_DEST_URI (see `make promo-up`)")
	}
	ctx := context.Background()
	src := newE2EClient(t, sourceURI)
	s3c := e2eS3Client(ctx, t)

	// --- Arrange a version WITH artifacts on the source server ---
	var exp struct {
		ExperimentID string `json:"experiment_id"`
	}
	require.NoError(t, src.Do(ctx, "POST", "experiments/create", nil,
		map[string]any{"name": uniq("promo-exp")}, &exp))

	var run struct {
		Run struct {
			Info struct {
				RunID       string `json:"run_id"`
				ArtifactURI string `json:"artifact_uri"`
			} `json:"info"`
		} `json:"run"`
	}
	require.NoError(t, src.Do(ctx, "POST", "runs/create", nil,
		map[string]any{"experiment_id": exp.ExperimentID}, &run))
	require.NotEmpty(t, run.Run.Info.ArtifactURI)

	// Upload a couple of model files to the run's artifact location in MinIO.
	modelURI := strings.TrimRight(run.Run.Info.ArtifactURI, "/") + "/model"
	putS3(ctx, t, s3c, modelURI+"/MLmodel", "flavors: {}\n")
	putS3(ctx, t, s3c, modelURI+"/conda.yaml", "name: m\n")

	model := uniq("promo-model")
	require.NoError(t, src.Do(ctx, "POST", "registered-models/create", nil,
		map[string]any{"name": model}, nil))
	var mv struct {
		ModelVersion struct {
			Version string `json:"version"`
		} `json:"model_version"`
	}
	require.NoError(t, src.Do(ctx, "POST", "model-versions/create", nil,
		map[string]any{"name": model, "source": modelURI, "run_id": run.Run.Info.RunID}, &mv))
	require.NotEmpty(t, mv.ModelVersion.Version)

	// --- Promote to the destination through the provider ---
	server := newE2EServer(t, destURI)
	var destSource string
	integration.LifeCycleTest{
		Resource: "mlflow:registry:ModelVersionPromotion",
		Create: integration.Operation{
			Inputs: propMap(map[string]any{
				"sourceTrackingUri": sourceURI,
				"sourceName":        model,
				"sourceVersion":     mv.ModelVersion.Version,
				"destName":          model,
			}),
			Hook: func(_, output property.Map) {
				assert.NotEmpty(t, output.Get("destVersion").AsString())
				destSource = output.Get("destSource").AsString()
				assert.Contains(t, destSource, "s3://mlflow-dst/")
			},
		},
	}.Run(t, server)

	// --- Verify the artifact bytes actually landed in the destination bucket ---
	require.Contains(t, destSource, "s3://mlflow-dst/")
	assert.True(t, s3ObjectExists(ctx, t, s3c, destSource+"/MLmodel"),
		"expected copied MLmodel under %s", destSource)
	assert.True(t, s3ObjectExists(ctx, t, s3c, destSource+"/conda.yaml"),
		"expected copied conda.yaml under %s", destSource)
}

// e2eS3Client builds a path-style S3 client (for MinIO/LocalStack); the endpoint
// comes from AWS_ENDPOINT_URL_S3 via the default config chain.
func e2eS3Client(ctx context.Context, t *testing.T) *s3.Client {
	t.Helper()
	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	require.NoError(t, err)
	return s3.NewFromConfig(cfg, func(o *s3.Options) { o.UsePathStyle = true })
}

func splitS3(t *testing.T, uri string) (bucket, key string) {
	t.Helper()
	u, err := url.Parse(uri)
	require.NoError(t, err)
	require.Equal(t, "s3", u.Scheme)
	return u.Host, strings.TrimPrefix(u.Path, "/")
}

func putS3(ctx context.Context, t *testing.T, c *s3.Client, uri, body string) {
	t.Helper()
	b, k := splitS3(t, uri)
	_, err := c.PutObject(ctx, &s3.PutObjectInput{Bucket: &b, Key: &k, Body: strings.NewReader(body)})
	require.NoError(t, err)
}

func s3ObjectExists(ctx context.Context, t *testing.T, c *s3.Client, uri string) bool {
	t.Helper()
	b, k := splitS3(t, uri)
	_, err := c.HeadObject(ctx, &s3.HeadObjectInput{Bucket: &b, Key: &k})
	return err == nil
}
