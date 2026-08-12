package registry

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/Baubap/pulumi-mlflow/provider/client"
)

// RegisteredModelTag manages a single key/value tag on a registered model that
// is owned elsewhere (for example, created by a training pipeline). Unlike the
// `tags` map on RegisteredModel it does NOT own or delete the model: it sets one
// tag on create, updates its value in place, and removes only that tag on
// delete. Use it to publish a fact about a model from another system — e.g. a
// deploy recording a service's serving host as the model's `model_serving_host`.
type RegisteredModelTag struct{}

// RegisteredModelTagArgs are the inputs for a RegisteredModelTag.
type RegisteredModelTagArgs struct {
	// Name is the registered model to tag. Changing it forces a replacement.
	Name string `pulumi:"name" provider:"replaceOnChanges"`
	// Key is the tag key. Changing it forces a replacement.
	Key string `pulumi:"key" provider:"replaceOnChanges"`
	// Value is the tag value. Updating it re-sets the tag in place.
	Value string `pulumi:"value"`
}

// RegisteredModelTagState is the persisted state for a RegisteredModelTag.
type RegisteredModelTagState struct {
	RegisteredModelTagArgs
}

const registeredModelTagDesc = "Manages a single key/value tag on a registered model that is owned elsewhere " +
	"(e.g. created by a training pipeline). Unlike the `tags` map on RegisteredModel it does not own the model: " +
	"it sets the tag on create, updates the value in place, and removes only that tag on delete. This mirrors the " +
	"`aws.ec2.Tag`-style pattern for managing one attribute of an externally-owned resource. Backed by the MLflow " +
	"`registered-models/set-tag` and `registered-models/delete-tag` endpoints.\n\n" +
	"{{% examples %}}\n## Example Usage\n{{% example %}}\n" +
	"{{< chooser language \"typescript,python\" >}}\n" +
	"{{% choosable language typescript %}}\n" +
	"```typescript\nimport * as mlflow from \"@baubap/mlflow\";\n\n" +
	"// Publish a service's host as a tag on a model created by the training pipeline.\n" +
	"new mlflow.registry.RegisteredModelTag(\"serving-host\", {\n" +
	"    name: \"fraud-detector\",\n    key: \"model_serving_host\",\n" +
	"    value: \"https://ml-fraud-prod.baubap.com\",\n});\n```\n" +
	"{{% /choosable %}}\n" +
	"{{% choosable language python %}}\n" +
	"```python\nimport baubap_mlflow as mlflow\n\n" +
	"mlflow.registry.RegisteredModelTag(\"serving-host\",\n" +
	"    name=\"fraud-detector\",\n    key=\"model_serving_host\",\n" +
	"    value=\"https://ml-fraud-prod.baubap.com\")\n```\n" +
	"{{% /choosable %}}\n" +
	"{{< /chooser >}}\n{{% /example %}}\n{{% /examples %}}\n\n" +
	"## Import\n\n```sh\npulumi import mlflow:registry:RegisteredModelTag serving-host <modelName>/<key>\n```"

// Annotate places the resource in the "registry" module.
func (r *RegisteredModelTag) Annotate(a infer.Annotator) {
	a.SetToken("registry", "RegisteredModelTag")
	a.Describe(r, registeredModelTagDesc)
}

func rmTagID(name, key string) string { return name + "/" + key }

func parseRMTagID(id string) (name, key string, ok bool) {
	i := strings.LastIndexByte(id, '/')
	if i <= 0 || i == len(id)-1 {
		return "", "", false
	}
	return id[:i], id[i+1:], true
}

func rmSetTag(ctx context.Context, api *client.Client, name, key, value string) error {
	body := map[string]any{"name": name, "key": key, "value": value}
	return api.Do(ctx, http.MethodPost, "registered-models/set-tag", nil, body, nil)
}

func rmDeleteTag(ctx context.Context, api *client.Client, name, key string) error {
	body := map[string]any{"name": name, "key": key}
	q := url.Values{"name": {name}, "key": {key}}
	return api.Do(ctx, http.MethodDelete, "registered-models/delete-tag", q, body, nil)
}

// Create sets the tag on the model.
func (RegisteredModelTag) Create(
	ctx context.Context, req infer.CreateRequest[RegisteredModelTagArgs],
) (infer.CreateResponse[RegisteredModelTagState], error) {
	in := req.Inputs
	state := RegisteredModelTagState{RegisteredModelTagArgs: in}
	id := rmTagID(in.Name, in.Key)
	if req.DryRun {
		return infer.CreateResponse[RegisteredModelTagState]{ID: id, Output: state}, nil
	}
	api := infer.GetConfig[client.Config](ctx).Client()
	if err := rmSetTag(ctx, api, in.Name, in.Key, in.Value); err != nil {
		return infer.CreateResponse[RegisteredModelTagState]{}, err
	}
	return infer.CreateResponse[RegisteredModelTagState]{ID: id, Output: state}, nil
}

// Read fetches the current tag value from the model; if the model or the tag is
// gone, it returns an empty ID so Pulumi reconciles the deletion.
func (RegisteredModelTag) Read(
	ctx context.Context, req infer.ReadRequest[RegisteredModelTagArgs, RegisteredModelTagState],
) (infer.ReadResponse[RegisteredModelTagArgs, RegisteredModelTagState], error) {
	name, key, ok := parseRMTagID(req.ID)
	if !ok {
		name, key = req.State.Name, req.State.Key
	}
	api := infer.GetConfig[client.Config](ctx).Client()
	dto, err := getRegisteredModel(ctx, api, name)
	if err != nil {
		if errors.Is(err, client.ErrNotFound) {
			return infer.ReadResponse[RegisteredModelTagArgs, RegisteredModelTagState]{}, nil
		}
		return infer.ReadResponse[RegisteredModelTagArgs, RegisteredModelTagState]{}, err
	}
	for _, t := range dto.Tags {
		if t.Key == key {
			args := RegisteredModelTagArgs{Name: name, Key: key, Value: t.Value}
			return infer.ReadResponse[RegisteredModelTagArgs, RegisteredModelTagState]{
				ID:     req.ID,
				Inputs: args,
				State:  RegisteredModelTagState{RegisteredModelTagArgs: args},
			}, nil
		}
	}
	return infer.ReadResponse[RegisteredModelTagArgs, RegisteredModelTagState]{}, nil
}

// Update re-sets the tag value (set-tag is an idempotent upsert).
func (RegisteredModelTag) Update(
	ctx context.Context, req infer.UpdateRequest[RegisteredModelTagArgs, RegisteredModelTagState],
) (infer.UpdateResponse[RegisteredModelTagState], error) {
	in := req.Inputs
	state := RegisteredModelTagState{RegisteredModelTagArgs: in}
	if req.DryRun {
		return infer.UpdateResponse[RegisteredModelTagState]{Output: state}, nil
	}
	api := infer.GetConfig[client.Config](ctx).Client()
	if err := rmSetTag(ctx, api, in.Name, in.Key, in.Value); err != nil {
		return infer.UpdateResponse[RegisteredModelTagState]{}, err
	}
	return infer.UpdateResponse[RegisteredModelTagState]{Output: state}, nil
}

// Delete removes only this tag from the model.
func (RegisteredModelTag) Delete(
	ctx context.Context, req infer.DeleteRequest[RegisteredModelTagState],
) (infer.DeleteResponse, error) {
	api := infer.GetConfig[client.Config](ctx).Client()
	if err := rmDeleteTag(ctx, api, req.State.Name, req.State.Key); err != nil &&
		!errors.Is(err, client.ErrNotFound) {
		return infer.DeleteResponse{}, err
	}
	return infer.DeleteResponse{}, nil
}
