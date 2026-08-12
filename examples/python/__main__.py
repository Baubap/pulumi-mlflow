"""Creates an MLflow experiment, a registered model and a model version."""

import pulumi
import pulumi_mlflow as mlflow

# A tracking experiment.
experiment = mlflow.Experiment(
    "demo",
    name="fraud-detection",
    tags={"team": "foundations", "env": "dev"},
)

# A registered model in the model registry.
model = mlflow.registry.RegisteredModel(
    "fraud",
    name="fraud-detector",
    description="Gradient-boosted fraud model",
    tags={"owner": "foundations"},
)

# A concrete version of that model.
v1 = mlflow.registry.ModelVersion(
    "fraud-v1",
    name=model.name,
    source="s3://models/fraud/1",
    description="First trained version",
)

# Point a mutable alias (preferred over deprecated stages) at the version.
mlflow.registry.RegisteredModelAlias(
    "fraud-champion",
    model_name=model.name,
    alias="champion",
    version=v1.version,
)

pulumi.export("experiment_id", experiment.experiment_id)
pulumi.export("model_version", v1.version)
