// Package provider assembles the MLflow Pulumi provider from its per-domain
// resource and function modules.
package provider

import (
	"fmt"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"

	"github.com/Baubap/pulumi-mlflow/provider/client"
)

// Name controls how this provider is referenced. The plugin binary is
// pulumi-resource-<Name>.
const Name string = "mlflow"

// Provider builds and returns the MLflow provider.
func Provider() p.Provider {
	prov, err := infer.NewProviderBuilder().
		WithDisplayName("MLflow").
		WithDescription("The MLflow provider for Pulumi manages MLflow experiments, model registry entries, "+
			"and access-control resources through the MLflow REST API. It supports MLflow tracking servers 2.x and 3.x.").
		WithPublisher("Baubap").
		WithRepository("https://github.com/Baubap/pulumi-mlflow").
		WithHomepage("https://github.com/Baubap/pulumi-mlflow").
		WithLicense("Apache-2.0").
		WithLogoURL("https://raw.githubusercontent.com/Baubap/pulumi-mlflow/main/docs/logo.svg").
		WithPluginDownloadURL("github://api.github.com/Baubap/pulumi-mlflow").
		WithKeywords("mlflow", "mlops", "machine-learning", "model-registry",
			"category/infrastructure", "kind/native").
		WithNamespace("baubap").
		WithGoImportPath("github.com/Baubap/pulumi-mlflow/sdk/go/mlflow").
		WithConfig(infer.Config(&client.Config{})).
		WithModuleMap(map[tokens.ModuleName]tokens.ModuleName{
			"provider": "index",
		}).
		WithResources(allResources()...).
		WithFunctions(allFunctions()...).
		Build()
	if err != nil {
		panic(fmt.Errorf("building mlflow provider: %w", err))
	}
	return prov
}
