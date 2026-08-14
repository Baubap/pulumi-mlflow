# Tag a model you don't own

Often the registered model itself is created and owned elsewhere — by a training pipeline, a data scientist's
notebook, or a different Pulumi stack — but you still want to attach a piece of governance metadata to it from your
infrastructure: which host currently serves it, which team owns it, a cost center, a compliance flag.

`RegisteredModelTag` manages a **single tag** on an existing registered model **without taking ownership of the
model**. It's the MLflow analogue of `aws.ec2.Tag`: Pulumi creates, updates and deletes exactly that one tag and
leaves the rest of the model alone. This is the pattern Baubap uses to record, from the serving stack, the endpoint
where each model is deployed.

## Reference the model, attach one tag

Look the model up read-only with the `getRegisteredModel` data source (so the program fails fast if it doesn't
exist), then declare the tag:

```typescript
import * as mlflow from "@baubap/mlflow";

// The model is owned elsewhere; reference it read-only just to confirm it exists.
const served = mlflow.registry.getRegisteredModel({ name: "fraud-detector" });

// Pulumi manages ONLY this tag — not the model. Record where it is served.
new mlflow.registry.RegisteredModelTag("fraud-serving-host", {
    name: "fraud-detector",
    key: "serving_host",
    value: "https://baubap-ml-mlflow-production.baubap.com",
});

export const owner = served.then(m => m.tags["owner"]);
```

The same in Python:

```python
import baubap_mlflow as mlflow

served = mlflow.registry.get_registered_model(name="fraud-detector")

mlflow.registry.RegisteredModelTag("fraud-serving-host",
    name="fraud-detector",
    key="serving_host",
    value="https://baubap-ml-mlflow-production.baubap.com")
```

…and in Go:

```go
package main

import (
	"github.com/Baubap/pulumi-mlflow/sdk/go/mlflow/registry"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		if _, err := registry.LookupRegisteredModel(ctx, &registry.LookupRegisteredModelArgs{
			Name: "fraud-detector",
		}); err != nil {
			return err
		}
		_, err := registry.NewRegisteredModelTag(ctx, "fraud-serving-host", &registry.RegisteredModelTagArgs{
			Name:  pulumi.String("fraud-detector"),
			Key:   pulumi.String("serving_host"),
			Value: pulumi.String("https://baubap-ml-mlflow-production.baubap.com"),
		})
		return err
	})
}
```

## Mutate the tag in place

`value` is mutable: change it and `pulumi up` **patches** the tag on the server — no replacement, one reviewable
diff. `name` and `key` identify the tag, so changing either is a replace (the old tag is deleted and a new one
created).

```diff
 new mlflow.registry.RegisteredModelTag("fraud-serving-host", {
     name: "fraud-detector",
     key: "serving_host",
-    value: "https://baubap-ml-mlflow-production.baubap.com",
+    value: "https://baubap-ml-mlflow-canary.baubap.com",
 });
```

Wiring this to a real deployment is the whole point — the `value` can be an output from the resource that serves
the model, so the tag always reflects where the model actually lives:

```typescript
// `service` is whatever exposes the model (an ECS service, an ALB, …).
new mlflow.registry.RegisteredModelTag("fraud-serving-host", {
    name: "fraud-detector",
    key: "serving_host",
    value: service.url,   // an Output<string>; the tag updates when the host changes
});
```

## Several tags

Declare one `RegisteredModelTag` per key. They're independent, so drift or a manual delete on one never touches the
others:

```typescript
const model = "fraud-detector";
for (const [key, value] of Object.entries({
    serving_host: "https://baubap-ml-mlflow-production.baubap.com",
    owner: "foundations",
    cost_center: "ml-platform",
})) {
    new mlflow.registry.RegisteredModelTag(`fraud-${key}`, { name: model, key, value });
}
```

> If instead you *own* the model in Pulumi, prefer the `tags` map on the `RegisteredModel` resource itself.
> `RegisteredModelTag` is for tagging a model whose lifecycle you don't manage.

## Import an existing tag

The resource ID is `<model>/<key>`:

```bash
pulumi import mlflow:registry:RegisteredModelTag fraud-serving-host fraud-detector/serving_host
```
