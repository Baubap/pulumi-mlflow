package registry

import (
	"strings"

	"github.com/pulumi/pulumi-go-provider/infer"
)

// md renders a rich schema description. "§" stands in for a backtick so the
// markdown — which uses ``` code fences and `inline code` — can live in a Go raw
// string literal (which cannot itself contain a backtick).
func md(s string) string { return strings.ReplaceAll(s, "§", "`") }

// ---- Resource descriptions --------------------------------------------------

var registeredModelDesc = md(`Manages a model in the [MLflow Model Registry](https://mlflow.org/docs/latest/ml/model-registry/) — a named container that groups the versions of a model.

The model §name§ is the resource identity; changing it replaces the resource. Only §description§ and §tags§ are updated in place.

REST endpoints: §/api/2.0/mlflow/registered-models/{create,get,update,delete,set-tag,delete-tag}§ — see the [MLflow REST API](https://mlflow.org/docs/latest/api_reference/rest-api.html#create-registeredmodel).

{{% examples %}}
## Example Usage
{{% example %}}
{{< chooser language "typescript,python,go" >}}
{{% choosable language typescript %}}
§§§typescript
import * as mlflow from "@baubap/mlflow";

const model = new mlflow.registry.RegisteredModel("fraud", {
    name: "fraud-detector",
    description: "Gradient-boosted fraud model",
    tags: { owner: "foundations" },
});
§§§
{{% /choosable %}}
{{% choosable language python %}}
§§§python
import baubap_mlflow as mlflow

model = mlflow.registry.RegisteredModel("fraud",
    name="fraud-detector",
    description="Gradient-boosted fraud model",
    tags={"owner": "foundations"})
§§§
{{% /choosable %}}
{{% choosable language go %}}
§§§go
model, err := registry.NewRegisteredModel(ctx, "fraud", &registry.RegisteredModelArgs{
    Name:        pulumi.String("fraud-detector"),
    Description: pulumi.String("Gradient-boosted fraud model"),
    Tags:        pulumi.StringMap{"owner": pulumi.String("foundations")},
})
§§§
{{% /choosable %}}
{{< /chooser >}}
{{% /example %}}
{{% /examples %}}

## Import

Registered models can be imported using the model name:

§§§sh
pulumi import mlflow:registry:RegisteredModel fraud fraud-detector
§§§
`)

var modelVersionDesc = md(`Manages a version of a [registered model](https://mlflow.org/docs/latest/ml/model-registry/) — an immutable snapshot of a model's artifacts.

The §source§, §runId§ and §runLink§ are set once: changing any of them replaces the version. Only §description§ and §tags§ update in place. On create, the provider waits until the version's status is §READY§.

> **Note:** §stage§ (None/Staging/Production/Archived) is **deprecated in MLflow 3**. Prefer [§RegisteredModelAlias§](/registry/registeredmodelalias) to drive deployments. Setting §stage§ against a 3.x server emits a warning.

REST endpoints: §/api/2.0/mlflow/model-versions/{create,get,update,delete,set-tag,delete-tag,transition-stage}§ — see the [MLflow REST API](https://mlflow.org/docs/latest/api_reference/rest-api.html#create-modelversion).

{{% examples %}}
## Example Usage
{{% example %}}
{{< chooser language "typescript,python,go" >}}
{{% choosable language typescript %}}
§§§typescript
import * as mlflow from "@baubap/mlflow";

const model = new mlflow.registry.RegisteredModel("fraud", { name: "fraud-detector" });

const v1 = new mlflow.registry.ModelVersion("fraud-v1", {
    name: model.name,
    source: "s3://models/fraud/1",
    description: "First trained version",
});
§§§
{{% /choosable %}}
{{% choosable language python %}}
§§§python
import baubap_mlflow as mlflow

model = mlflow.registry.RegisteredModel("fraud", name="fraud-detector")

v1 = mlflow.registry.ModelVersion("fraud-v1",
    name=model.name,
    source="s3://models/fraud/1",
    description="First trained version")
§§§
{{% /choosable %}}
{{% choosable language go %}}
§§§go
model, _ := registry.NewRegisteredModel(ctx, "fraud", &registry.RegisteredModelArgs{
    Name: pulumi.String("fraud-detector"),
})

v1, err := registry.NewModelVersion(ctx, "fraud-v1", &registry.ModelVersionArgs{
    Name:        model.Name,
    Source:      pulumi.String("s3://models/fraud/1"),
    Description: pulumi.String("First trained version"),
})
§§§
{{% /choosable %}}
{{< /chooser >}}
{{% /example %}}
{{% /examples %}}

## Import

Model versions can be imported using §<model-name>/<version>§:

§§§sh
pulumi import mlflow:registry:ModelVersion fraud-v1 fraud-detector/1
§§§
`)

var registeredModelAliasDesc = md(`Manages a named [alias](https://mlflow.org/docs/latest/ml/model-registry/#deploy-and-organize-models-with-aliases-and-tags) on a registered model that points at a specific version (for example §champion§ → version 3).

Aliases are mutable pointers and are the **recommended** way to drive deployments; they supersede the deprecated model *stages*. §version§ is updatable — changing it re-points the alias to a different version — while §modelName§ and §alias§ changes replace the resource.

REST endpoints: §/api/2.0/mlflow/registered-models/alias§ (set/delete/get) — see the [MLflow REST API](https://mlflow.org/docs/latest/api_reference/rest-api.html#set-registered-model-alias).

{{% examples %}}
## Example Usage
{{% example %}}
{{< chooser language "typescript,python,go" >}}
{{% choosable language typescript %}}
§§§typescript
import * as mlflow from "@baubap/mlflow";

// Point the "champion" alias at a model version to promote it.
new mlflow.registry.RegisteredModelAlias("champion", {
    modelName: "fraud-detector",
    alias: "champion",
    version: "3",
});
§§§
{{% /choosable %}}
{{% choosable language python %}}
§§§python
import baubap_mlflow as mlflow

# Point the "champion" alias at a model version to promote it.
mlflow.registry.RegisteredModelAlias("champion",
    model_name="fraud-detector",
    alias="champion",
    version="3")
§§§
{{% /choosable %}}
{{% choosable language go %}}
§§§go
_, err := registry.NewRegisteredModelAlias(ctx, "champion", &registry.RegisteredModelAliasArgs{
    ModelName: pulumi.String("fraud-detector"),
    Alias:     pulumi.String("champion"),
    Version:   pulumi.String("3"),
})
§§§
{{% /choosable %}}
{{< /chooser >}}
{{% /example %}}
{{% /examples %}}

## Import

Aliases can be imported using §<model-name>/<alias>§:

§§§sh
pulumi import mlflow:registry:RegisteredModelAlias champion fraud-detector/champion
§§§
`)

// ---- Function (data source) descriptions ------------------------------------

var getRegisteredModelDesc = md(`Looks up a registered model in the [Model Registry](https://mlflow.org/docs/latest/ml/model-registry/) by name.

REST: §/api/2.0/mlflow/registered-models/get§.

{{% examples %}}
## Example Usage
{{% example %}}
{{< chooser language "typescript,python" >}}
{{% choosable language typescript %}}
§§§typescript
import * as mlflow from "@baubap/mlflow";

const model = mlflow.registry.getRegisteredModel({ name: "fraud-detector" });
§§§
{{% /choosable %}}
{{% choosable language python %}}
§§§python
import baubap_mlflow as mlflow

model = mlflow.registry.get_registered_model(name="fraud-detector")
§§§
{{% /choosable %}}
{{< /chooser >}}
{{% /example %}}
{{% /examples %}}
`)

var searchRegisteredModelsDesc = md(`Searches registered models with an optional MLflow [filter string](https://mlflow.org/docs/latest/api_reference/rest-api.html#search-registeredmodels).

REST: §/api/2.0/mlflow/registered-models/search§.

{{% examples %}}
## Example Usage
{{% example %}}
{{< chooser language "typescript,python" >}}
{{% choosable language typescript %}}
§§§typescript
import * as mlflow from "@baubap/mlflow";

const models = mlflow.registry.searchRegisteredModels({ filter: "name LIKE 'fraud%'" });
§§§
{{% /choosable %}}
{{% choosable language python %}}
§§§python
import baubap_mlflow as mlflow

models = mlflow.registry.search_registered_models(filter="name LIKE 'fraud%'")
§§§
{{% /choosable %}}
{{< /chooser >}}
{{% /example %}}
{{% /examples %}}
`)

var getLatestVersionsDesc = md(`Returns the latest version of a registered model per stage (or overall).

REST: §/api/2.0/mlflow/registered-models/get-latest-versions§.

{{% examples %}}
## Example Usage
{{% example %}}
{{< chooser language "typescript,python" >}}
{{% choosable language typescript %}}
§§§typescript
import * as mlflow from "@baubap/mlflow";

const latest = mlflow.registry.getLatestVersions({ name: "fraud-detector" });
§§§
{{% /choosable %}}
{{% choosable language python %}}
§§§python
import baubap_mlflow as mlflow

latest = mlflow.registry.get_latest_versions(name="fraud-detector")
§§§
{{% /choosable %}}
{{< /chooser >}}
{{% /example %}}
{{% /examples %}}
`)

var getModelVersionDesc = md(`Looks up a specific model version by model name and version number.

REST: §/api/2.0/mlflow/model-versions/get§.

{{% examples %}}
## Example Usage
{{% example %}}
{{< chooser language "typescript,python" >}}
{{% choosable language typescript %}}
§§§typescript
import * as mlflow from "@baubap/mlflow";

const mv = mlflow.registry.getModelVersion({ name: "fraud-detector", version: "1" });
§§§
{{% /choosable %}}
{{% choosable language python %}}
§§§python
import baubap_mlflow as mlflow

mv = mlflow.registry.get_model_version(name="fraud-detector", version="1")
§§§
{{% /choosable %}}
{{< /chooser >}}
{{% /example %}}
{{% /examples %}}
`)

var searchModelVersionsDesc = md(`Searches model versions with an optional MLflow filter string.

REST: §/api/2.0/mlflow/model-versions/search§.

{{% examples %}}
## Example Usage
{{% example %}}
{{< chooser language "typescript,python" >}}
{{% choosable language typescript %}}
§§§typescript
import * as mlflow from "@baubap/mlflow";

const versions = mlflow.registry.searchModelVersions({ filter: "name='fraud-detector'" });
§§§
{{% /choosable %}}
{{% choosable language python %}}
§§§python
import baubap_mlflow as mlflow

versions = mlflow.registry.search_model_versions(filter="name='fraud-detector'")
§§§
{{% /choosable %}}
{{< /chooser >}}
{{% /example %}}
{{% /examples %}}
`)

var getModelVersionByAliasDesc = md(`Resolves an alias on a registered model to the model version it currently points at — useful for reading which version is, for example, §champion§.

REST: §/api/2.0/mlflow/registered-models/alias§ (get).

{{% examples %}}
## Example Usage
{{% example %}}
{{< chooser language "typescript,python" >}}
{{% choosable language typescript %}}
§§§typescript
import * as mlflow from "@baubap/mlflow";

const champion = mlflow.registry.getModelVersionByAlias({ name: "fraud-detector", alias: "champion" });
§§§
{{% /choosable %}}
{{% choosable language python %}}
§§§python
import baubap_mlflow as mlflow

champion = mlflow.registry.get_model_version_by_alias(name="fraud-detector", alias="champion")
§§§
{{% /choosable %}}
{{< /chooser >}}
{{% /example %}}
{{% /examples %}}
`)

var getModelVersionDownloadUriDesc = md(`Returns the artifact download URI for a model version.

REST: §/api/2.0/mlflow/model-versions/get-download-uri§.

{{% examples %}}
## Example Usage
{{% example %}}
{{< chooser language "typescript,python" >}}
{{% choosable language typescript %}}
§§§typescript
import * as mlflow from "@baubap/mlflow";

const uri = mlflow.registry.getModelVersionDownloadUri({ name: "fraud-detector", version: "1" });
§§§
{{% /choosable %}}
{{% choosable language python %}}
§§§python
import baubap_mlflow as mlflow

uri = mlflow.registry.get_model_version_download_uri(name="fraud-detector", version="1")
§§§
{{% /choosable %}}
{{< /chooser >}}
{{% /example %}}
{{% /examples %}}
`)

// ---- Property descriptions --------------------------------------------------

// Annotate documents RegisteredModel input fields.
func (a *RegisteredModelArgs) Annotate(an infer.Annotator) {
	an.Describe(&a.Name, "Unique name of the registered model. This is the resource identity; changing it replaces the resource.")
	an.Describe(&a.Description, "Optional human-readable description. Updated in place.")
	an.Describe(&a.Tags, "Optional key/value metadata tags. Synced in place via set-tag/delete-tag.")
}

// Annotate documents RegisteredModel output fields.
func (s *RegisteredModelState) Annotate(an infer.Annotator) {
	an.Describe(&s.CreationTimestamp, "Model creation time, in epoch milliseconds.")
	an.Describe(&s.LastUpdatedTimestamp, "Last update time, in epoch milliseconds.")
}

// Annotate documents RegisteredModelAlias input fields.
func (a *RegisteredModelAliasArgs) Annotate(an infer.Annotator) {
	an.Describe(&a.ModelName, "Registered model the alias belongs to. Changing it replaces the resource.")
	an.Describe(&a.Alias, "Alias name, e.g. \"champion\". Changing it replaces the resource.")
	an.Describe(&a.Version, "Model version the alias points at. Updating it re-points the alias in place.")
}

// Annotate documents ModelVersion output fields.
func (s *ModelVersionState) Annotate(an infer.Annotator) {
	an.Describe(&s.Version, "Server-assigned version number (immutable).")
	an.Describe(&s.CurrentStage, "The version's current stage (deprecated in MLflow 3; prefer aliases).")
	an.Describe(&s.Status, "Registration status; the provider waits for READY on create.")
	an.Describe(&s.Aliases, "Aliases currently pointing at this version.")
	an.Describe(&s.CreationTimestamp, "Creation time, in epoch milliseconds.")
	an.Describe(&s.LastUpdatedTimestamp, "Last update time, in epoch milliseconds.")
}
