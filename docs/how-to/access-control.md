# Set up access control

The `auth` module manages MLflow's built-in authentication: users and per-experiment / per-registered-model
permissions. These resources talk to the MLflow auth REST API, which only exists when the tracking server runs
with the auth app enabled.

## Enable the auth app on the server

```bash
mlflow server --app-name basic-auth \
  --backend-store-uri sqlite:///mlflow.db \
  --host 0.0.0.0 --port 5000
```

If the auth endpoints are not available, the `auth` resources return an error telling you to enable
`mlflow.server.auth`.

## Manage users and permissions

```typescript
import * as mlflow from "@baubap/mlflow";

// A user. The password is write-only and stored as a Pulumi secret.
const alice = new mlflow.auth.User("alice", {
    username: "alice",
    password: config.requireSecret("alicePassword"),
});

// Grant alice EDIT on a specific experiment.
new mlflow.auth.ExperimentPermission("alice-fraud-exp", {
    experimentId: experiment.experimentId,
    username: alice.username,
    permission: "EDIT",
});

// …and READ on a registered model.
new mlflow.auth.RegisteredModelPermission("alice-fraud-model", {
    name: "fraud-detector",
    username: alice.username,
    permission: "READ",
});
```

```python
import pulumi
import baubap_mlflow as mlflow

config = pulumi.Config()

alice = mlflow.auth.User("alice",
    username="alice",
    password=config.require_secret("alicePassword"))

mlflow.auth.ExperimentPermission("alice-fraud-exp",
    experiment_id=experiment.experiment_id,
    username=alice.username,
    permission="EDIT")
```

## Permission levels

`permission` accepts one of: **`READ`**, **`EDIT`**, **`MANAGE`**, **`NO_PERMISSIONS`**. It is updatable in place —
changing it runs an update, not a replacement. The `username` and target (`experimentId` / `name`) are part of the
identity, so changing them replaces the permission.

## Import existing users and permissions

```bash
pulumi import mlflow:auth:User alice alice
pulumi import mlflow:auth:ExperimentPermission alice-fraud-exp <experimentId>/alice
pulumi import mlflow:auth:RegisteredModelPermission alice-fraud-model fraud-detector/alice
```
