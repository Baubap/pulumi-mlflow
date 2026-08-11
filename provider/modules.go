package provider

import (
	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/Baubap/pulumi-mlflow/provider/auth"
	"github.com/Baubap/pulumi-mlflow/provider/registry"
	"github.com/Baubap/pulumi-mlflow/provider/tracking"
)

// moduleResources returns the resources registered by every domain module.
func moduleResources() []infer.InferredResource {
	return concat(
		tracking.Resources(),
		registry.Resources(),
		auth.Resources(),
	)
}

// moduleFunctions returns the functions registered by every domain module.
func moduleFunctions() []infer.InferredFunction {
	return concatFns(
		tracking.Functions(),
		registry.Functions(),
		auth.Functions(),
	)
}

func concat(groups ...[]infer.InferredResource) []infer.InferredResource {
	var out []infer.InferredResource
	for _, g := range groups {
		out = append(out, g...)
	}
	return out
}

func concatFns(groups ...[]infer.InferredFunction) []infer.InferredFunction {
	var out []infer.InferredFunction
	for _, g := range groups {
		out = append(out, g...)
	}
	return out
}
