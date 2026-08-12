// Package tracking implements MLflow tracking resources and data sources
// (experiments and read-only lookups). Resources are placed in the provider's
// "index" module via SetToken in each resource's Annotate.
package tracking

import "github.com/pulumi/pulumi-go-provider/infer"

// Resources returns the tracking module's Pulumi resources.
func Resources() []infer.InferredResource {
	return []infer.InferredResource{
		infer.Resource(Experiment{}),
	}
}

// Functions returns the tracking module's Pulumi functions (data sources).
func Functions() []infer.InferredFunction {
	return []infer.InferredFunction{
		infer.Function(GetExperiment{}),
		infer.Function(GetExperimentByName{}),
		infer.Function(SearchExperiments{}),
	}
}
