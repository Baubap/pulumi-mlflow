package tracking

import (
	"context"
	"net/http"
	"net/url"

	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/Baubap/pulumi-mlflow/provider/client"
)

// ExperimentResult is the shape returned by the experiment data-source functions.
type ExperimentResult struct {
	// Server-assigned unique identifier of the experiment.
	ExperimentID string `pulumi:"experimentId"`
	// Name of the experiment.
	Name string `pulumi:"name"`
	// Artifact storage location of the experiment.
	ArtifactLocation string `pulumi:"artifactLocation"`
	// Lifecycle stage: "active" or "deleted".
	LifecycleStage string `pulumi:"lifecycleStage"`
	// Key/value tags attached to the experiment.
	Tags map[string]string `pulumi:"tags"`
}

func resultFromDTO(d *experimentDTO) ExperimentResult {
	return ExperimentResult{
		ExperimentID:     d.ExperimentID,
		Name:             d.Name,
		ArtifactLocation: d.ArtifactLocation,
		LifecycleStage:   d.LifecycleStage,
		Tags:             client.KVToMap(d.Tags),
	}
}

// GetExperiment looks up an experiment by its ID.
type GetExperiment struct{}

// GetExperimentArgs are the inputs to getExperiment.
type GetExperimentArgs struct {
	// ID of the experiment to look up.
	ExperimentID string `pulumi:"experimentId"`
}

// Annotate names the function getExperiment in the "index" module.
func (g *GetExperiment) Annotate(a infer.Annotator) {
	a.SetToken("index", "getExperiment")
	a.Describe(g, getExperimentDesc)
}

// Invoke resolves the experiment by ID.
func (GetExperiment) Invoke(
	ctx context.Context, req infer.FunctionRequest[GetExperimentArgs],
) (infer.FunctionResponse[ExperimentResult], error) {
	cl := infer.GetConfig[client.Config](ctx).Client()
	dto, err := fetchExperiment(ctx, cl, req.Input.ExperimentID)
	if err != nil {
		return infer.FunctionResponse[ExperimentResult]{}, err
	}
	return infer.FunctionResponse[ExperimentResult]{Output: resultFromDTO(dto)}, nil
}

// GetExperimentByName looks up an experiment by its name.
type GetExperimentByName struct{}

// GetExperimentByNameArgs are the inputs to getExperimentByName.
type GetExperimentByNameArgs struct {
	// Name of the experiment to look up.
	Name string `pulumi:"name"`
}

// Annotate names the function getExperimentByName in the "index" module.
func (g *GetExperimentByName) Annotate(a infer.Annotator) {
	a.SetToken("index", "getExperimentByName")
	a.Describe(g, getExperimentByNameDesc)
}

// Invoke resolves the experiment by name.
func (GetExperimentByName) Invoke(
	ctx context.Context, req infer.FunctionRequest[GetExperimentByNameArgs],
) (infer.FunctionResponse[ExperimentResult], error) {
	cl := infer.GetConfig[client.Config](ctx).Client()
	q := url.Values{}
	q.Set("experiment_name", req.Input.Name)
	var resp struct {
		Experiment experimentDTO `json:"experiment"`
	}
	if err := cl.Do(ctx, http.MethodGet, "experiments/get-by-name", q, nil, &resp); err != nil {
		return infer.FunctionResponse[ExperimentResult]{}, err
	}
	return infer.FunctionResponse[ExperimentResult]{Output: resultFromDTO(&resp.Experiment)}, nil
}

// SearchExperiments searches experiments with an optional filter.
type SearchExperiments struct{}

// SearchExperimentsArgs are the inputs to searchExperiments.
type SearchExperimentsArgs struct {
	// MLflow filter expression, e.g. `tags.team = "ml"`. Empty returns all.
	Filter *string `pulumi:"filter,optional"`
	// Maximum number of experiments to return.
	MaxResults *int `pulumi:"maxResults,optional"`
}

// SearchExperimentsResult is the output of searchExperiments.
type SearchExperimentsResult struct {
	// The experiments matching the search.
	Experiments []ExperimentResult `pulumi:"experiments"`
}

// Annotate names the function searchExperiments in the "index" module.
func (g *SearchExperiments) Annotate(a infer.Annotator) {
	a.SetToken("index", "searchExperiments")
	a.Describe(g, searchExperimentsDesc)
}

// Invoke runs the experiment search.
func (SearchExperiments) Invoke(
	ctx context.Context, req infer.FunctionRequest[SearchExperimentsArgs],
) (infer.FunctionResponse[SearchExperimentsResult], error) {
	cl := infer.GetConfig[client.Config](ctx).Client()
	body := map[string]any{}
	if req.Input.Filter != nil && *req.Input.Filter != "" {
		body["filter"] = *req.Input.Filter
	}
	if req.Input.MaxResults != nil {
		body["max_results"] = *req.Input.MaxResults
	}
	var resp struct {
		Experiments []experimentDTO `json:"experiments"`
	}
	if err := cl.Do(ctx, http.MethodPost, "experiments/search", nil, body, &resp); err != nil {
		return infer.FunctionResponse[SearchExperimentsResult]{}, err
	}
	out := SearchExperimentsResult{Experiments: make([]ExperimentResult, 0, len(resp.Experiments))}
	for i := range resp.Experiments {
		out.Experiments = append(out.Experiments, resultFromDTO(&resp.Experiments[i]))
	}
	return infer.FunctionResponse[SearchExperimentsResult]{Output: out}, nil
}
