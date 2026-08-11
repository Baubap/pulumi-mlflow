# Promote a model with aliases

MLflow model-version **stages** (`Staging`, `Production`) are deprecated in MLflow 3. The recommended way to drive
deployments is with **aliases** — mutable, named pointers from a registered model to a specific version. This guide
shows a blue/green rollout using `champion` and `challenger` aliases managed by Pulumi.

## Why aliases

- An alias like `champion` is a stable name your serving layer reads; you repoint it at a new version to release.
- Repointing is a single, reviewable Pulumi change (`version: v1.version` → `version: v2.version`).
- Unlike stages, you can have as many aliases as you need (`champion`, `challenger`, `canary`, …).

## Roll out a new version

```typescript
import * as mlflow from "@baubap/mlflow";

const model = new mlflow.registry.RegisteredModel("fraud", { name: "fraud-detector" });

// The version currently in production.
const v1 = new mlflow.registry.ModelVersion("fraud-v1", {
    name: model.name,
    source: "s3://models/fraud/1",
});

// A new candidate.
const v2 = new mlflow.registry.ModelVersion("fraud-v2", {
    name: model.name,
    source: "s3://models/fraud/2",
});

// `champion` serves production traffic; `challenger` is the candidate under evaluation.
new mlflow.registry.RegisteredModelAlias("champion", {
    modelName: model.name,
    alias: "champion",
    version: v1.version,
});

new mlflow.registry.RegisteredModelAlias("challenger", {
    modelName: model.name,
    alias: "challenger",
    version: v2.version,
});
```

To **promote** the challenger, change the `champion` alias to point at `v2` and run `pulumi up`:

```diff
 new mlflow.registry.RegisteredModelAlias("champion", {
     modelName: model.name,
     alias: "champion",
-    version: v1.version,
+    version: v2.version,
 });
```

Because the alias name (`champion`) is unchanged and only `version` changes, Pulumi repoints it in place — no
resource is recreated. Your serving layer, which resolves `models:/fraud-detector@champion`, immediately picks up
the new version.

## Read the current champion

```typescript
const current = mlflow.registry.getModelVersionByAlias({
    name: "fraud-detector",
    alias: "champion",
});
export const championVersion = current.then(c => c.version);
```
