import * as mlflow from "@baubap/mlflow";

// A tracking experiment.
const experiment = new mlflow.Experiment("demo", {
    name: "fraud-detection",
    tags: { team: "foundations", env: "dev" },
});

// A registered model in the model registry.
const model = new mlflow.registry.RegisteredModel("fraud", {
    name: "fraud-detector",
    description: "Gradient-boosted fraud model",
    tags: { owner: "foundations" },
});

// A concrete version of that model.
const v1 = new mlflow.registry.ModelVersion("fraud-v1", {
    name: model.name,
    source: "s3://models/fraud/1",
    description: "First trained version",
});

// Point a mutable alias (preferred over deprecated stages) at the version.
new mlflow.registry.RegisteredModelAlias("fraud-champion", {
    modelName: model.name,
    alias: "champion",
    version: v1.version,
});

export const experimentId = experiment.experimentId;
export const modelVersion = v1.version;
