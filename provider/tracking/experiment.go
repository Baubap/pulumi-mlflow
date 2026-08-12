package tracking

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/Baubap/pulumi-mlflow/provider/client"
)

// Experiment manages an MLflow experiment: a named container that groups runs
// and their artifacts. It maps to the /api/2.0/mlflow/experiments REST endpoints.
type Experiment struct{}

// ExperimentArgs are the inputs to an Experiment.
type ExperimentArgs struct {
	// Human-readable, unique name of the experiment.
	Name string `pulumi:"name"`
	// Root artifact storage location for the experiment (e.g. an s3:// URI). Set
	// once at creation time; changing it forces the experiment to be replaced.
	ArtifactLocation *string `pulumi:"artifactLocation,optional" provider:"replaceOnChanges"`
	// Key/value tags attached to the experiment. Note: MLflow exposes no
	// delete-experiment-tag endpoint, so removing a tag in place is not supported.
	Tags map[string]string `pulumi:"tags,optional"`
}

// ExperimentState is the persisted state of an Experiment.
type ExperimentState struct {
	ExperimentArgs
	// Server-assigned unique identifier of the experiment.
	ExperimentID string `pulumi:"experimentId"`
	// Lifecycle stage of the experiment: "active" or "deleted".
	LifecycleStage string `pulumi:"lifecycleStage"`
	// Resolved artifact storage location as reported by the tracking server.
	ArtifactUri string `pulumi:"artifactUri"`
}

// Annotate places the resource in the provider's "index" module.
func (e *Experiment) Annotate(a infer.Annotator) {
	a.SetToken("index", "Experiment")
	a.Describe(e, experimentDesc)
}

// Annotate documents the Experiment input fields.
func (e *ExperimentArgs) Annotate(a infer.Annotator) {
	a.Describe(&e.Name, "Human-readable, unique name of the experiment. Must be unique across active "+
		"experiments on the tracking server; updating it renames the experiment in place.")
	a.Describe(&e.ArtifactLocation, "Root artifact storage location for the experiment (e.g. an `s3://` URI). "+
		"Optional — the server assigns a default when omitted. **Immutable:** set once at creation; changing it "+
		"forces the experiment to be replaced.")
	a.Describe(&e.Tags, "Key/value tags attached to the experiment. Updated in place via `set-experiment-tag`. "+
		"**Note:** MLflow has no delete-experiment-tag endpoint, so removing a tag in place is not supported.")
}

// Annotate documents the Experiment output fields.
func (e *ExperimentState) Annotate(a infer.Annotator) {
	a.Describe(&e.ExperimentID, "Server-assigned unique identifier of the experiment.")
	a.Describe(&e.LifecycleStage, "Lifecycle stage of the experiment: \"active\" or \"deleted\".")
	a.Describe(&e.ArtifactUri, "Resolved artifact storage location as reported by the tracking server.")
}

// experimentDTO is the MLflow REST representation of an experiment.
type experimentDTO struct {
	ExperimentID     string            `json:"experiment_id"`
	Name             string            `json:"name"`
	ArtifactLocation string            `json:"artifact_location"`
	LifecycleStage   string            `json:"lifecycle_stage"`
	Tags             []client.KeyValue `json:"tags"`
}

func fetchExperiment(ctx context.Context, cl *client.Client, id string) (*experimentDTO, error) {
	q := url.Values{}
	q.Set("experiment_id", id)
	var resp struct {
		Experiment experimentDTO `json:"experiment"`
	}
	if err := cl.Do(ctx, http.MethodGet, "experiments/get", q, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Experiment, nil
}

func experimentStateFrom(inputs ExperimentArgs, dto *experimentDTO) ExperimentState {
	return ExperimentState{
		ExperimentArgs: inputs,
		ExperimentID:   dto.ExperimentID,
		LifecycleStage: dto.LifecycleStage,
		ArtifactUri:    dto.ArtifactLocation,
	}
}

// Create registers a new experiment.
func (Experiment) Create(
	ctx context.Context, req infer.CreateRequest[ExperimentArgs],
) (infer.CreateResponse[ExperimentState], error) {
	inputs := req.Inputs
	if req.DryRun {
		return infer.CreateResponse[ExperimentState]{Output: ExperimentState{ExperimentArgs: inputs}}, nil
	}

	cl := infer.GetConfig[client.Config](ctx).Client()
	body := map[string]any{"name": inputs.Name}
	if inputs.ArtifactLocation != nil && *inputs.ArtifactLocation != "" {
		body["artifact_location"] = *inputs.ArtifactLocation
	}
	if kv := client.TagsToKV(inputs.Tags); kv != nil {
		body["tags"] = kv
	}

	var created struct {
		ExperimentID string `json:"experiment_id"`
	}
	if err := cl.Do(ctx, http.MethodPost, "experiments/create", nil, body, &created); err != nil {
		return infer.CreateResponse[ExperimentState]{}, err
	}
	dto, err := fetchExperiment(ctx, cl, created.ExperimentID)
	if err != nil {
		return infer.CreateResponse[ExperimentState]{}, err
	}
	return infer.CreateResponse[ExperimentState]{ID: created.ExperimentID, Output: experimentStateFrom(inputs, dto)}, nil
}

// Read syncs Pulumi state with the experiment's actual state.
func (Experiment) Read(
	ctx context.Context, req infer.ReadRequest[ExperimentArgs, ExperimentState],
) (infer.ReadResponse[ExperimentArgs, ExperimentState], error) {
	cl := infer.GetConfig[client.Config](ctx).Client()
	dto, err := fetchExperiment(ctx, cl, req.ID)
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			// Empty ID tells Pulumi the resource no longer exists.
			return infer.ReadResponse[ExperimentArgs, ExperimentState]{}, nil
		}
		return infer.ReadResponse[ExperimentArgs, ExperimentState]{}, err
	}

	inputs := req.Inputs
	inputs.Name = dto.Name
	// Assign unconditionally: when every tag is removed server-side dto.Tags is
	// empty and KVToMap returns nil, so a guarded assignment would keep a stale
	// map and hide the deletion from refresh.
	inputs.Tags = client.KVToMap(dto.Tags)
	// artifactLocation is echoed from the program input; the server-resolved value
	// is surfaced via ArtifactUri.
	return infer.ReadResponse[ExperimentArgs, ExperimentState]{
		ID:     req.ID,
		Inputs: inputs,
		State:  experimentStateFrom(inputs, dto),
	}, nil
}

// Update applies name and tag changes in place. Changes to artifactLocation are
// handled by replacement (see the replaceOnChanges tag).
func (Experiment) Update(
	ctx context.Context, req infer.UpdateRequest[ExperimentArgs, ExperimentState],
) (infer.UpdateResponse[ExperimentState], error) {
	news := req.Inputs
	olds := req.State
	if req.DryRun {
		merged := olds
		merged.ExperimentArgs = news
		return infer.UpdateResponse[ExperimentState]{Output: merged}, nil
	}

	cl := infer.GetConfig[client.Config](ctx).Client()
	id := olds.ExperimentID

	if news.Name != olds.Name {
		if err := cl.Do(ctx, http.MethodPost, "experiments/update", nil,
			map[string]any{"experiment_id": id, "new_name": news.Name}, nil); err != nil {
			return infer.UpdateResponse[ExperimentState]{}, err
		}
	}

	upserts, removals := client.DiffTags(olds.Tags, news.Tags)
	for k, v := range upserts {
		if err := cl.Do(ctx, http.MethodPost, "experiments/set-experiment-tag", nil,
			map[string]any{"experiment_id": id, "key": k, "value": v}, nil); err != nil {
			return infer.UpdateResponse[ExperimentState]{}, err
		}
	}
	if len(removals) > 0 {
		p.GetLogger(ctx).Warningf(
			"MLflow has no delete-experiment-tag endpoint; tags %v cannot be removed in place and remain on experiment %s",
			removals, id)
	}

	dto, err := fetchExperiment(ctx, cl, id)
	if err != nil {
		return infer.UpdateResponse[ExperimentState]{}, err
	}
	return infer.UpdateResponse[ExperimentState]{Output: experimentStateFrom(news, dto)}, nil
}

// Delete removes the experiment (soft-delete on the MLflow side).
func (Experiment) Delete(
	ctx context.Context, req infer.DeleteRequest[ExperimentState],
) (infer.DeleteResponse, error) {
	cl := infer.GetConfig[client.Config](ctx).Client()
	err := cl.Do(ctx, http.MethodPost, "experiments/delete", nil,
		map[string]any{"experiment_id": req.State.ExperimentID}, nil)
	if err != nil && !errors.Is(err, client.ErrNotFound) {
		return infer.DeleteResponse{}, err
	}
	return infer.DeleteResponse{}, nil
}
