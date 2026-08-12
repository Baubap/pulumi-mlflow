// Package registry implements MLflow Model Registry resources and data sources
// (registered models, model versions and aliases). Resources are placed in the
// provider's "registry" module via SetToken in each resource's Annotate.
package registry

import "github.com/pulumi/pulumi-go-provider/infer"

// Resources returns the registry module's Pulumi resources.
func Resources() []infer.InferredResource {
	return []infer.InferredResource{
		infer.Resource(RegisteredModel{}),
		infer.Resource(ModelVersion{}),
		infer.Resource(RegisteredModelAlias{}),
		infer.Resource(RegisteredModelTag{}),
	}
}

// Functions returns the registry module's Pulumi functions (data sources).
func Functions() []infer.InferredFunction {
	return []infer.InferredFunction{
		infer.Function(GetRegisteredModel{}),
		infer.Function(SearchRegisteredModels{}),
		infer.Function(GetLatestVersions{}),
		infer.Function(GetModelVersion{}),
		infer.Function(SearchModelVersions{}),
		infer.Function(GetModelVersionByAlias{}),
		infer.Function(GetModelVersionDownloadUri{}),
	}
}
