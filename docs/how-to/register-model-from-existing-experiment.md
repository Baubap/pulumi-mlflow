# Register a model from an existing experiment

A data scientist — or a training job — has already created an experiment, and now you want to codify the registry
entry that goes with it: a registered model, its first version, and a `champion` alias, linked back to that
experiment. Use the `getExperimentByName` data source to look the experiment up, then build the registry resources
around it.

## Look up the experiment, build the registry entry

```typescript
import * as mlflow from "@baubap/mlflow";

// The experiment already exists (created during training). Reference it read-only.
const experiment = mlflow.getExperimentByName({ name: "fraud-detection" });

// Create the registered model and tag it back to its experiment.
const model = new mlflow.registry.RegisteredModel("fraud", {
    name: "fraud-detector",
    description: "Gradient-boosted fraud model",
    tags: {
        experiment_id: experiment.then(e => e.experimentId),
        source_experiment: "fraud-detection",
    },
});

// A first version, and a mutable alias your serving layer resolves.
const v1 = new mlflow.registry.ModelVersion("fraud-v1", {
    name: model.name,
    source: "s3://models/fraud/1",
});

new mlflow.registry.RegisteredModelAlias("fraud-champion", {
    modelName: model.name,
    alias: "champion",
    version: v1.version,
});

export const experimentId = experiment.then(e => e.experimentId);
```

The same in Python:

```python
import baubap_mlflow as mlflow

experiment = mlflow.get_experiment_by_name(name="fraud-detection")

model = mlflow.registry.RegisteredModel("fraud",
    name="fraud-detector",
    description="Gradient-boosted fraud model",
    tags={
        "experiment_id": experiment.experiment_id,
        "source_experiment": "fraud-detection",
    })

v1 = mlflow.registry.ModelVersion("fraud-v1",
    name=model.name,
    source="s3://models/fraud/1")

mlflow.registry.RegisteredModelAlias("fraud-champion",
    model_name=model.name,
    alias="champion",
    version=v1.version)
```

…and in Go:

```go
package main

import (
	"github.com/Baubap/pulumi-mlflow/sdk/go/mlflow"
	"github.com/Baubap/pulumi-mlflow/sdk/go/mlflow/registry"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		experiment, err := mlflow.GetExperimentByName(ctx, &mlflow.GetExperimentByNameArgs{
			Name: "fraud-detection",
		})
		if err != nil {
			return err
		}
		model, err := registry.NewRegisteredModel(ctx, "fraud", &registry.RegisteredModelArgs{
			Name:        pulumi.String("fraud-detector"),
			Description: pulumi.String("Gradient-boosted fraud model"),
			Tags: pulumi.StringMap{
				"experiment_id":     pulumi.String(experiment.ExperimentId),
				"source_experiment": pulumi.String("fraud-detection"),
			},
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

`getExperimentByName` is a **read-only** data source: it resolves the experiment's id (and tags, artifact location,
lifecycle stage) at preview time and never modifies it. The registry resources above are what Pulumi creates and
owns.

## Adopt the experiment too

If you'd rather have Pulumi *manage* the experiment as well — not just read it — import it once, then create a real
`Experiment` resource and depend on it directly instead of the data source:

```bash
pulumi import mlflow:index:Experiment fraud-detection <experiment-id>
```

```typescript
const experiment = new mlflow.Experiment("fraud-detection", {
    name: "fraud-detection",
    tags: { team: "foundations" },
});

const model = new mlflow.registry.RegisteredModel("fraud", {
    name: "fraud-detector",
    tags: { experiment_id: experiment.experimentId },
});
```

Now a change to the experiment and the model version roll out together in one `pulumi up`.
