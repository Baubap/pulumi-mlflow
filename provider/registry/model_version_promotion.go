package registry

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/Baubap/pulumi-mlflow/provider/client"
)

// ModelVersionPromotion copies a model version from one MLflow tracking server
// to another (e.g. testing -> production), including its artifacts, so the
// destination is self-contained ("gold copy"): deleting the source can never
// break the destination.
//
// Metadata (experiment, run, model version, tags) is replicated over the MLflow
// REST API. Artifact bytes are copied by a scheme-aware backend selected from
// the source URI: s3:// is copied directly with the AWS SDK (the process running
// the provider must have credentials + access to both buckets); mlflow-artifacts:/
// is proxied through the tracking server (no cloud credentials needed). This
// mirrors how the MLflow client itself dispatches on the artifact URI scheme.
//
// The destination version number is assigned by the destination registry and is
// surfaced as an output; point a RegisteredModelAlias at it to promote.
type ModelVersionPromotion struct{}

// ModelVersionPromotionArgs are the inputs for a ModelVersionPromotion.
type ModelVersionPromotionArgs struct {
	// SourceTrackingUri is the source MLflow tracking server (e.g. the testing
	// server). Changing it forces a replacement.
	SourceTrackingUri string `pulumi:"sourceTrackingUri" provider:"replaceOnChanges"`
	// SourceName is the registered model name on the source server. Changing it forces a replacement.
	SourceName string `pulumi:"sourceName" provider:"replaceOnChanges"`
	// SourceVersion is the model version to promote from the source server. Changing it forces a replacement.
	SourceVersion string `pulumi:"sourceVersion" provider:"replaceOnChanges"`
	// DestName is the registered model name to create on the destination server.
	// Defaults to SourceName. Changing it forces a replacement.
	DestName *string `pulumi:"destName,optional" provider:"replaceOnChanges"`
	// DestExperiment is the experiment name to place the copied run under on the
	// destination. Defaults to the source run's experiment name. Changing it forces a replacement.
	DestExperiment *string `pulumi:"destExperiment,optional" provider:"replaceOnChanges"`
	// CopyRunData replicates the source run's params, metrics and tags onto the
	// destination run. Defaults to true.
	CopyRunData *bool `pulumi:"copyRunData,optional"`
}

// ModelVersionPromotionState is the persisted state for a ModelVersionPromotion.
type ModelVersionPromotionState struct {
	ModelVersionPromotionArgs
	// DestVersion is the version number assigned by the destination registry.
	DestVersion string `pulumi:"destVersion"`
	// DestRunId is the run created on the destination that holds the copied artifacts.
	DestRunId string `pulumi:"destRunId"`
	// DestSource is the artifact URI of the promoted version on the destination.
	DestSource string `pulumi:"destSource"`
}

const modelVersionPromotionDesc = "Copies a model version from one MLflow tracking server to another " +
	"(e.g. testing -> production), **including its artifacts**, so the destination is self-contained: deleting " +
	"the source can never break the destination. Metadata (experiment, run, model version) is replicated over the " +
	"MLflow REST API; artifact bytes are copied by a scheme-aware backend (`s3://` directly via the AWS SDK — the " +
	"process needs credentials to both stores; `mlflow-artifacts:/` proxied through the server, no credentials). " +
	"The destination version number is assigned by the destination registry and returned as `destVersion` — point a " +
	"`RegisteredModelAlias` at it to promote.\n\n" +
	"{{% examples %}}\n## Example Usage\n{{% example %}}\n" +
	"{{< chooser language \"typescript,python\" >}}\n" +
	"{{% choosable language typescript %}}\n" +
	"```typescript\nimport * as mlflow from \"@baubap/mlflow\";\n\n" +
	"const prod = new mlflow.Provider(\"prod\", { trackingUri: \"https://mlflow-prod.example.com\" });\n\n" +
	"const promoted = new mlflow.registry.ModelVersionPromotion(\"fraud-v3\", {\n" +
	"    sourceTrackingUri: \"https://mlflow-test.example.com\",\n" +
	"    sourceName: \"fraud-detector\",\n    sourceVersion: \"3\",\n}, { provider: prod });\n\n" +
	"new mlflow.registry.RegisteredModelAlias(\"fraud-prod\", {\n" +
	"    modelName: \"fraud-detector\", alias: \"production\", version: promoted.destVersion,\n" +
	"}, { provider: prod });\n```\n" +
	"{{% /choosable %}}\n" +
	"{{% choosable language python %}}\n" +
	"```python\nimport baubap_mlflow as mlflow\n\n" +
	"prod = mlflow.Provider(\"prod\", tracking_uri=\"https://mlflow-prod.example.com\")\n\n" +
	"promoted = mlflow.registry.ModelVersionPromotion(\"fraud-v3\",\n" +
	"    source_tracking_uri=\"https://mlflow-test.example.com\",\n" +
	"    source_name=\"fraud-detector\",\n    source_version=\"3\",\n    opts=pulumi.ResourceOptions(provider=prod))\n```\n" +
	"{{% /choosable %}}\n" +
	"{{< /chooser >}}\n{{% /example %}}\n{{% /examples %}}"

// Annotate places the resource in the "registry" module and documents inputs.
func (r *ModelVersionPromotion) Annotate(a infer.Annotator) {
	a.SetToken("registry", "ModelVersionPromotion")
	a.Describe(r, modelVersionPromotionDesc)
}

func promotionID(destName, destVersion string) string { return destName + "/" + destVersion }

// Create performs the cross-server deep copy.
func (ModelVersionPromotion) Create(
	ctx context.Context, req infer.CreateRequest[ModelVersionPromotionArgs],
) (infer.CreateResponse[ModelVersionPromotionState], error) {
	in := req.Inputs
	destName := in.SourceName
	if in.DestName != nil && *in.DestName != "" {
		destName = *in.DestName
	}
	if req.DryRun {
		return infer.CreateResponse[ModelVersionPromotionState]{
			Output: ModelVersionPromotionState{ModelVersionPromotionArgs: in},
		}, nil
	}

	dst := infer.GetConfig[client.Config](ctx).Client()
	src, err := client.NewClient(in.SourceTrackingUri, "", "", "", false)
	if err != nil {
		return infer.CreateResponse[ModelVersionPromotionState]{}, fmt.Errorf("source client: %w", err)
	}

	// 1. Read the source version (gives the artifact source URI + run id).
	srcMv, err := getModelVersion(ctx, src, in.SourceName, in.SourceVersion)
	if err != nil {
		return infer.CreateResponse[ModelVersionPromotionState]{}, fmt.Errorf("reading source %s/%s: %w", in.SourceName, in.SourceVersion, err)
	}

	// 2. Resolve the source run (for the experiment + run data) when present.
	var srcRun *runDTO
	if srcMv.RunID != "" {
		srcRun, err = getRun(ctx, src, srcMv.RunID)
		if err != nil && !errors.Is(err, client.ErrNotFound) {
			return infer.CreateResponse[ModelVersionPromotionState]{}, fmt.Errorf("reading source run %s: %w", srcMv.RunID, err)
		}
	}

	// 3. Resolve the destination experiment name and ensure it exists.
	destExpName := "promoted-models"
	if in.DestExperiment != nil && *in.DestExperiment != "" {
		destExpName = *in.DestExperiment
	} else if srcRun != nil {
		if name, err := getExperimentName(ctx, src, srcRun.Info.ExperimentID); err == nil && name != "" {
			destExpName = name
		}
	}
	destExpID, err := getOrCreateExperiment(ctx, dst, destExpName)
	if err != nil {
		return infer.CreateResponse[ModelVersionPromotionState]{}, fmt.Errorf("dest experiment %q: %w", destExpName, err)
	}

	// 4. Create a destination run to hold the gold artifacts.
	destRun, err := createRun(ctx, dst, destExpID, map[string]string{
		"promoted_from":         in.SourceTrackingUri,
		"promoted_from_model":   in.SourceName,
		"promoted_from_version": in.SourceVersion,
	})
	if err != nil {
		return infer.CreateResponse[ModelVersionPromotionState]{}, fmt.Errorf("creating dest run: %w", err)
	}

	// 5. Replicate run params/metrics/tags (best-effort, opt-out via copyRunData).
	if (in.CopyRunData == nil || *in.CopyRunData) && srcRun != nil {
		if err := logRunData(ctx, dst, destRun.Info.RunID, srcRun.Data); err != nil {
			return infer.CreateResponse[ModelVersionPromotionState]{}, fmt.Errorf("copying run data: %w", err)
		}
	}

	// 6. Copy artifact bytes to the dest run's artifact root, preserving the
	//    model's sub-path (e.g. ".../artifacts/model").
	var subPath string
	if srcRun != nil && srcRun.Info.ArtifactURI != "" && strings.HasPrefix(srcMv.Source, srcRun.Info.ArtifactURI) {
		subPath = strings.TrimPrefix(strings.TrimPrefix(srcMv.Source, srcRun.Info.ArtifactURI), "/")
	} else {
		subPath = lastPathSegment(srcMv.Source)
	}
	destSource := strings.TrimRight(destRun.Info.ArtifactURI, "/") + "/" + subPath
	if err := copyArtifactsFn(ctx, dst, srcMv.Source, destSource); err != nil {
		return infer.CreateResponse[ModelVersionPromotionState]{}, fmt.Errorf("copying artifacts %s -> %s: %w", srcMv.Source, destSource, err)
	}

	// 7. Ensure the destination registered model exists, then create the version.
	if err := ensureRegisteredModel(ctx, dst, destName); err != nil {
		return infer.CreateResponse[ModelVersionPromotionState]{}, fmt.Errorf("dest registered model %q: %w", destName, err)
	}
	runLink := strings.TrimRight(in.SourceTrackingUri, "/") + "/#/experiments/" + srcRunExperiment(srcRun) + "/runs/" + srcMv.RunID
	destMv, err := createModelVersion(ctx, dst, destName, destSource, destRun.Info.RunID, runLink, srcMv.Tags)
	if err != nil {
		return infer.CreateResponse[ModelVersionPromotionState]{}, fmt.Errorf("creating dest model version: %w", err)
	}

	state := ModelVersionPromotionState{
		ModelVersionPromotionArgs: in,
		DestVersion:               destMv.Version,
		DestRunId:                 destRun.Info.RunID,
		DestSource:                destSource,
	}
	return infer.CreateResponse[ModelVersionPromotionState]{ID: promotionID(destName, destMv.Version), Output: state}, nil
}

// Read confirms the promoted version still exists on the destination.
func (ModelVersionPromotion) Read(
	ctx context.Context, req infer.ReadRequest[ModelVersionPromotionArgs, ModelVersionPromotionState],
) (infer.ReadResponse[ModelVersionPromotionArgs, ModelVersionPromotionState], error) {
	st := req.State
	destName := st.SourceName
	if st.DestName != nil && *st.DestName != "" {
		destName = *st.DestName
	}
	if destName == "" || st.DestVersion == "" {
		return infer.ReadResponse[ModelVersionPromotionArgs, ModelVersionPromotionState]{ID: req.ID, Inputs: req.Inputs, State: st}, nil
	}
	dst := infer.GetConfig[client.Config](ctx).Client()
	if _, err := getModelVersion(ctx, dst, destName, st.DestVersion); err != nil {
		if errors.Is(err, client.ErrNotFound) {
			return infer.ReadResponse[ModelVersionPromotionArgs, ModelVersionPromotionState]{}, nil
		}
		return infer.ReadResponse[ModelVersionPromotionArgs, ModelVersionPromotionState]{}, err
	}
	return infer.ReadResponse[ModelVersionPromotionArgs, ModelVersionPromotionState]{ID: req.ID, Inputs: req.Inputs, State: st}, nil
}

// Delete is a no-op: the promoted version and its gold artifacts are retained on
// the destination on purpose (destroying the promotion must not delete prod).
func (ModelVersionPromotion) Delete(
	_ context.Context, _ infer.DeleteRequest[ModelVersionPromotionState],
) (infer.DeleteResponse, error) {
	return infer.DeleteResponse{}, nil
}

func srcRunExperiment(r *runDTO) string {
	if r == nil {
		return ""
	}
	return r.Info.ExperimentID
}

func lastPathSegment(uri string) string {
	uri = strings.TrimRight(uri, "/")
	if i := strings.LastIndexByte(uri, '/'); i >= 0 && i < len(uri)-1 {
		return uri[i+1:]
	}
	return "model"
}

// ---- MLflow REST helpers used by the promotion deep copy --------------------

type runInfoDTO struct {
	RunID        string `json:"run_id"`
	ExperimentID string `json:"experiment_id"`
	ArtifactURI  string `json:"artifact_uri"`
	Status       string `json:"status"`
}

type metricDTO struct {
	Key       string  `json:"key"`
	Value     float64 `json:"value"`
	Timestamp int64   `json:"timestamp"`
	Step      int64   `json:"step"`
}

type runDataDTO struct {
	Metrics []metricDTO       `json:"metrics"`
	Params  []client.KeyValue `json:"params"`
	Tags    []client.KeyValue `json:"tags"`
}

type runDTO struct {
	Info runInfoDTO `json:"info"`
	Data runDataDTO `json:"data"`
}

func getRun(ctx context.Context, api *client.Client, runID string) (*runDTO, error) {
	q := url.Values{"run_id": {runID}}
	var resp struct {
		Run runDTO `json:"run"`
	}
	if err := api.Do(ctx, http.MethodGet, "runs/get", q, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Run, nil
}

func getExperimentName(ctx context.Context, api *client.Client, experimentID string) (string, error) {
	q := url.Values{"experiment_id": {experimentID}}
	var resp struct {
		Experiment struct {
			Name string `json:"name"`
		} `json:"experiment"`
	}
	if err := api.Do(ctx, http.MethodGet, "experiments/get", q, nil, &resp); err != nil {
		return "", err
	}
	return resp.Experiment.Name, nil
}

func getOrCreateExperiment(ctx context.Context, api *client.Client, name string) (string, error) {
	q := url.Values{"experiment_name": {name}}
	var got struct {
		Experiment struct {
			ExperimentID string `json:"experiment_id"`
		} `json:"experiment"`
	}
	err := api.Do(ctx, http.MethodGet, "experiments/get-by-name", q, nil, &got)
	if err == nil && got.Experiment.ExperimentID != "" {
		return got.Experiment.ExperimentID, nil
	}
	if err != nil && !errors.Is(err, client.ErrNotFound) {
		return "", err
	}
	var created struct {
		ExperimentID string `json:"experiment_id"`
	}
	if err := api.Do(ctx, http.MethodPost, "experiments/create", nil, map[string]any{"name": name}, &created); err != nil {
		return "", err
	}
	return created.ExperimentID, nil
}

func createRun(ctx context.Context, api *client.Client, experimentID string, tags map[string]string) (*runDTO, error) {
	body := map[string]any{"experiment_id": experimentID}
	if len(tags) > 0 {
		body["tags"] = client.TagsToKV(tags)
	}
	var resp struct {
		Run runDTO `json:"run"`
	}
	if err := api.Do(ctx, http.MethodPost, "runs/create", nil, body, &resp); err != nil {
		return nil, err
	}
	return &resp.Run, nil
}

func logRunData(ctx context.Context, api *client.Client, runID string, data runDataDTO) error {
	// log-batch caps at 1000 metrics / 100 params / 100 tags per call; chunk params/tags/metrics.
	params := data.Params
	tags := data.Tags
	metrics := data.Metrics
	// First call carries params+tags (small in practice) plus a metrics slice.
	for i := 0; ; i += 1000 {
		body := map[string]any{"run_id": runID}
		if i == 0 {
			if len(params) > 0 {
				body["params"] = params
			}
			if len(tags) > 0 {
				body["tags"] = tags
			}
		}
		end := i + 1000
		if end > len(metrics) {
			end = len(metrics)
		}
		if i < len(metrics) {
			body["metrics"] = metrics[i:end]
		}
		if _, ok := body["params"]; !ok {
			if _, ok := body["tags"]; !ok {
				if _, ok := body["metrics"]; !ok {
					break
				}
			}
		}
		if err := api.Do(ctx, http.MethodPost, "runs/log-batch", nil, body, nil); err != nil {
			return err
		}
		if end >= len(metrics) {
			break
		}
	}
	return nil
}

func ensureRegisteredModel(ctx context.Context, api *client.Client, name string) error {
	err := api.Do(ctx, http.MethodPost, "registered-models/create", nil, map[string]any{"name": name}, nil)
	if err != nil && errors.Is(err, client.ErrAlreadyExists) {
		return nil
	}
	return err
}

func createModelVersion(
	ctx context.Context, api *client.Client, name, source, runID, runLink string, tags []client.KeyValue,
) (*modelVersionDTO, error) {
	body := map[string]any{"name": name, "source": source}
	if runID != "" {
		body["run_id"] = runID
	}
	if runLink != "" {
		body["run_link"] = runLink
	}
	if len(tags) > 0 {
		body["tags"] = tags
	}
	var resp struct {
		ModelVersion modelVersionDTO `json:"model_version"`
	}
	if err := api.Do(ctx, http.MethodPost, "model-versions/create", nil, body, &resp); err != nil {
		return nil, err
	}
	return &resp.ModelVersion, nil
}
