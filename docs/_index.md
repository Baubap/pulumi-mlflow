---
title: MLflow
meta_desc: The MLflow provider for Pulumi manages MLflow experiments, model registry entries, and access control through the MLflow REST API.
layout: package
---

The MLflow provider for Pulumi lets you declaratively manage [MLflow](https://mlflow.org) resources — experiments,
the model registry (registered models, model versions, and aliases), and access control (users and permissions) —
through the MLflow REST API. It works with MLflow tracking servers **2.x and 3.x**.

Use it to codify the parts of your MLflow estate that are infrastructure: which experiments and registered models
exist, which version an alias like `champion` points at, and who can read or edit them — reviewed and rolled out
the same way as the rest of your Pulumi-managed platform.

## Overview

The provider is organized into three modules:

| Module | Resources | What it manages |
|---|---|---|
| `index` (tracking) | `Experiment` | Experiments that group runs and artifacts |
| `registry` | `RegisteredModel`, `ModelVersion`, `RegisteredModelAlias`, `RegisteredModelTag` | The Model Registry: models, versions, deployment aliases, and single-tag management |
| `auth` | `User`, `ExperimentPermission`, `RegisteredModelPermission` | Access control (requires the `mlflow.server.auth` app) |

Each module also ships read-only **functions** (data sources) such as `getRegisteredModel`,
`getModelVersionByAlias`, and `searchExperiments`.

## Example

{{< chooser language "typescript,python,go" >}}

{{% choosable language typescript %}}
```typescript
import * as mlflow from "@baubap/mlflow";

const experiment = new mlflow.Experiment("demo", {
    name: "fraud-detection",
    tags: { team: "foundations" },
});

const model = new mlflow.registry.RegisteredModel("fraud", {
    name: "fraud-detector",
    description: "Gradient-boosted fraud model",
});

const v1 = new mlflow.registry.ModelVersion("fraud-v1", {
    name: model.name,
    source: "s3://models/fraud/1",
});

// Point deployments at a mutable alias instead of a deprecated stage.
new mlflow.registry.RegisteredModelAlias("fraud-champion", {
    modelName: model.name,
    alias: "champion",
    version: v1.version,
});
```
{{% /choosable %}}

{{% choosable language python %}}
```python
import baubap_mlflow as mlflow

experiment = mlflow.Experiment("demo",
    name="fraud-detection",
    tags={"team": "foundations"})

model = mlflow.registry.RegisteredModel("fraud",
    name="fraud-detector",
    description="Gradient-boosted fraud model")

v1 = mlflow.registry.ModelVersion("fraud-v1",
    name=model.name,
    source="s3://models/fraud/1")

# Point deployments at a mutable alias instead of a deprecated stage.
mlflow.registry.RegisteredModelAlias("fraud-champion",
    model_name=model.name,
    alias="champion",
    version=v1.version)
```
{{% /choosable %}}

{{% choosable language go %}}
```go
package main

import (
	"github.com/Baubap/pulumi-mlflow/sdk/go/mlflow"
	"github.com/Baubap/pulumi-mlflow/sdk/go/mlflow/registry"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		_, err := mlflow.NewExperiment(ctx, "demo", &mlflow.ExperimentArgs{
			Name: pulumi.String("fraud-detection"),
			Tags: pulumi.StringMap{"team": pulumi.String("foundations")},
		})
		if err != nil {
			return err
		}
		model, err := registry.NewRegisteredModel(ctx, "fraud", &registry.RegisteredModelArgs{
			Name:        pulumi.String("fraud-detector"),
			Description: pulumi.String("Gradient-boosted fraud model"),
		})
		if err != nil {
			return err
		}
		v1, err := registry.NewModelVersion(ctx, "fraud-v1", &registry.ModelVersionArgs{
			Name:   model.Name,
			Source: pulumi.String("s3://models/fraud/1"),
		})
		if err != nil {
			return err
		}
		_, err = registry.NewRegisteredModelAlias(ctx, "fraud-champion", &registry.RegisteredModelAliasArgs{
			ModelName: model.Name,
			Alias:     pulumi.String("champion"),
			Version:   v1.Version,
		})
		return err
	})
}
```
{{% /choosable %}}

{{< /chooser >}}

## Concepts

- **Experiments** group training runs and their artifacts. The provider manages the experiment itself (name,
  artifact location, tags) — runs and metrics are logged at training time and are not Pulumi-managed.
- **Registered models** are named entries in the Model Registry; **model versions** are immutable snapshots of a
  model's artifacts (`source`). A version's `source`, `runId` and `runLink` are set once — changing them replaces
  the version.
- **Aliases** are mutable, named pointers (e.g. `champion`, `challenger`) from a registered model to a specific
  version. They are the recommended way to drive deployments and **supersede the deprecated model *stages***
  (`Staging`/`Production`), which are deprecated in MLflow 3. Prefer `RegisteredModelAlias` over `ModelVersion.stage`.
- **Access control** (`User`, `ExperimentPermission`, `RegisteredModelPermission`) is only available when the
  tracking server runs with the built-in auth app — see [Installation & Configuration](installation-configuration).

## MLflow 2.x and 3.x

The declarative REST surface these resources use is identical across MLflow 2.x and 3.x, so the same program works
against either. The provider detects the server version at runtime and uses it only to warn when you set a
deprecated field (for example, `ModelVersion.stage` on a 3.x server).

## Guides

- [Promote a model with aliases](https://github.com/Baubap/pulumi-mlflow/blob/main/docs/how-to/promote-model-with-aliases.md) — blue/green model rollouts using `champion`/`challenger` aliases.
- [Set up access control](https://github.com/Baubap/pulumi-mlflow/blob/main/docs/how-to/access-control.md) — manage users and per-experiment / per-model permissions.

## Importing existing resources

Every resource supports `pulumi import`, so you can bring an already-existing MLflow estate under management. See
the **Import** section on each resource's page (for example, `pulumi import mlflow:registry:RegisteredModel fraud fraud-detector`).
