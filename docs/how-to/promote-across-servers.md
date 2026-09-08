# Promote a model across servers

When you run **separate MLflow tracking servers per environment** (e.g. a testing server where models are trained and
registered, and a production server that serves them), promoting a model means getting a version — **and its
artifacts** — from one server onto the other. `ModelVersionPromotion` does that deep copy so the destination is
**self-contained**: it holds its own copy of the artifacts ("gold copy"), so deleting or cleaning up the source can
never break production.

## What it does

Given a source version, `ModelVersionPromotion` (running against the **destination** provider):

1. reads the source version + run over the MLflow REST API;
2. recreates the experiment and run on the destination and replicates params / metrics / tags;
3. copies the artifact bytes to the destination's artifact store;
4. creates the destination model version pointing at the copied artifacts.

The destination version number is assigned by the destination registry and returned as `destVersion` — point a
`RegisteredModelAlias` at it to promote.

## Usage

```typescript
import * as mlflow from "@baubap/mlflow";

const prod = new mlflow.Provider("prod", { trackingUri: "https://mlflow-prod.example.com" });

// Deep-copy testing v3 → production (artifacts included).
const promoted = new mlflow.registry.ModelVersionPromotion("fraud-v3", {
    sourceTrackingUri: "https://mlflow-test.example.com",
    sourceName: "fraud-detector",
    sourceVersion: "3",
    // destName defaults to sourceName; destExperiment defaults to the source run's experiment
}, { provider: prod });

// Promotion = point the production alias at the freshly-copied version.
new mlflow.registry.RegisteredModelAlias("fraud-prod", {
    modelName: "fraud-detector",
    alias: "production",
    version: promoted.destVersion,
}, { provider: prod });

export const promotedVersion = promoted.destVersion;
```

The same in Python:

```python
import baubap_mlflow as mlflow
import pulumi

prod = mlflow.Provider("prod", tracking_uri="https://mlflow-prod.example.com")

promoted = mlflow.registry.ModelVersionPromotion("fraud-v3",
    source_tracking_uri="https://mlflow-test.example.com",
    source_name="fraud-detector",
    source_version="3",
    opts=pulumi.ResourceOptions(provider=prod))

mlflow.registry.RegisteredModelAlias("fraud-prod",
    model_name="fraud-detector",
    alias="production",
    version=promoted.dest_version,
    opts=pulumi.ResourceOptions(provider=prod))
```

## How artifacts are copied

The artifact backend is chosen from the source URI scheme, mirroring the MLflow client:

- **`s3://`** — copied directly with the AWS SDK (server-side S3 copy; bytes never transit the machine running
  Pulumi). The process running `pulumi up` **must have credentials and access to both buckets** (typically a single
  role with read on the source bucket and write on the destination). This is the default for servers started with
  `--no-serve-artifacts` + `--default-artifact-root s3://…`, where MLflow reports real `s3://` paths.
- **`mlflow-artifacts:/`** — (proxied artifacts; roadmap) copied over the tracking server's artifact REST API, so
  **no cloud credentials** are needed. Requires both servers started with `--serve-artifacts`.

## Notes

- **Immutable inputs.** `sourceTrackingUri`, `sourceName`, `sourceVersion` and `destName` are replace-on-change — a
  promotion maps one source version to one destination copy.
- **Delete is a no-op.** Destroying the promotion does **not** delete the copied version or its artifacts on the
  destination — production is retained on purpose.
- **Idempotency.** Re-running overwrites the destination artifact objects at the same path; a new destination version
  is created only on first create.
