package provider

import "github.com/pulumi/pulumi-go-provider/infer"

// allResources aggregates the inferred resources contributed by every module.
// Each domain package (tracking, registry, auth) exposes a Resources() slice
// that is appended here during integration.
func allResources() []infer.InferredResource {
	var rs []infer.InferredResource
	rs = append(rs, moduleResources()...)
	return rs
}

// allFunctions aggregates the inferred functions contributed by every module.
func allFunctions() []infer.InferredFunction {
	var fs []infer.InferredFunction
	fs = append(fs, moduleFunctions()...)
	return fs
}
