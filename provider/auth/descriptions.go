package auth

// Rich Registry documentation for the auth module. infer only surfaces text
// passed to Annotator.Describe, so per-resource "Example Usage" and "Import"
// sections live here and are wired into each Annotate. The example code snippets
// are backtick-free, so they are raw string literals; the helpers add the
// Markdown code fences and Pulumi Registry chooser shortcodes.

// authNote is appended to every auth description: these endpoints only exist
// when the server runs the auth app.
const authNote = "\n\n> **Requires the MLflow auth app.** These resources call the " +
	"[MLflow authentication REST API](https://mlflow.org/docs/latest/auth/rest-api.html), which is only " +
	"available when the tracking server runs with the `mlflow.server.auth` app enabled " +
	"(e.g. `mlflow server --app-name basic-auth`).\n"

// exampleBlock wraps per-language snippets in the Registry's examples/chooser shortcodes.
func exampleBlock(ts, py, golang string) string {
	return "\n{{% examples %}}\n## Example Usage\n\n{{% example %}}\n" +
		"{{< chooser language \"typescript,python,go\" >}}\n" +
		"{{% choosable language typescript %}}\n```typescript\n" + ts + "\n```\n{{% /choosable %}}\n" +
		"{{% choosable language python %}}\n```python\n" + py + "\n```\n{{% /choosable %}}\n" +
		"{{% choosable language go %}}\n```go\n" + golang + "\n```\n{{% /choosable %}}\n" +
		"{{< /chooser >}}\n{{% /example %}}\n{{% /examples %}}\n"
}

// importBlock renders a Registry "Import" section.
func importBlock(token, id string) string {
	return "\n## Import\n\nImport an existing resource with its id:\n\n```sh\n$ pulumi import " +
		token + " example " + id + "\n```\n"
}

// ---- User -------------------------------------------------------------------

const userTS = `import * as mlflow from "@baubap/mlflow";

const jdoe = new mlflow.auth.User("jdoe", {
    username: "jdoe",
    password: config.requireSecret("jdoePassword"),
    isAdmin: false,
});`

const userPY = `import pulumi
import baubap_mlflow as mlflow

config = pulumi.Config()
jdoe = mlflow.auth.User("jdoe",
    username="jdoe",
    password=config.require_secret("jdoePassword"),
    is_admin=False)`

const userGO = `_, err := auth.NewUser(ctx, "jdoe", &auth.UserArgs{
    Username: pulumi.String("jdoe"),
    Password: cfg.RequireSecret("jdoePassword"),
    IsAdmin:  pulumi.Bool(false),
})`

var userDesc = "Manages a user in the MLflow authentication database. The `password` is write-only and " +
	"is never read back from the server." + authNote +
	exampleBlock(userTS, userPY, userGO) +
	importBlock("mlflow:auth:User", "<username>")

// ---- ExperimentPermission ---------------------------------------------------

const experimentPermissionTS = `import * as mlflow from "@baubap/mlflow";

const readAccess = new mlflow.auth.ExperimentPermission("read-access", {
    experimentId: "0",
    username: "jdoe",
    permission: "READ",
});`

const experimentPermissionPY = `import baubap_mlflow as mlflow

read_access = mlflow.auth.ExperimentPermission("read-access",
    experiment_id="0",
    username="jdoe",
    permission="READ")`

const experimentPermissionGo = `_, err := auth.NewExperimentPermission(ctx, "read-access", &auth.ExperimentPermissionArgs{
    ExperimentId: pulumi.String("0"),
    Username:     pulumi.String("jdoe"),
    Permission:   pulumi.String("READ"),
})`

var experimentPermissionDesc = "Grants a user a permission level on an MLflow experiment. Valid `permission` " +
	"values are `READ`, `EDIT`, `MANAGE` and `NO_PERMISSIONS`; only `permission` can be updated in place " +
	"(changing the experiment or user replaces the grant)." + authNote +
	exampleBlock(experimentPermissionTS, experimentPermissionPY, experimentPermissionGo) +
	importBlock("mlflow:auth:ExperimentPermission", "<experimentId>/<username>")

// ---- RegisteredModelPermission ----------------------------------------------

const registeredModelPermissionTS = `import * as mlflow from "@baubap/mlflow";

const manage = new mlflow.auth.RegisteredModelPermission("manage", {
    name: "fraud-detector",
    username: "jdoe",
    permission: "MANAGE",
});`

const registeredModelPermissionPY = `import baubap_mlflow as mlflow

manage = mlflow.auth.RegisteredModelPermission("manage",
    name="fraud-detector",
    username="jdoe",
    permission="MANAGE")`

const registeredModelPermissionGo = `_, err := auth.NewRegisteredModelPermission(ctx, "manage", &auth.RegisteredModelPermissionArgs{
    Name:       pulumi.String("fraud-detector"),
    Username:   pulumi.String("jdoe"),
    Permission: pulumi.String("MANAGE"),
})`

var registeredModelPermissionDesc = "Grants a user a permission level on an MLflow registered model. Valid " +
	"`permission` values are `READ`, `EDIT`, `MANAGE` and `NO_PERMISSIONS`; only `permission` can be updated " +
	"in place (changing the model or user replaces the grant)." + authNote +
	exampleBlock(registeredModelPermissionTS, registeredModelPermissionPY, registeredModelPermissionGo) +
	importBlock("mlflow:auth:RegisteredModelPermission", "<name>/<username>")

// ---- Functions --------------------------------------------------------------

const getUserTS = `import * as mlflow from "@baubap/mlflow";

const jdoe = mlflow.auth.getUser({ username: "jdoe" });`

const getUserPY = `import baubap_mlflow as mlflow

jdoe = mlflow.auth.get_user(username="jdoe")`

const getUserGo = `jdoe, err := auth.GetUser(ctx, &auth.GetUserArgs{Username: "jdoe"})`

var getUserDesc = "Looks up an MLflow user by username, returning its admin flag and id." + authNote +
	exampleBlock(getUserTS, getUserPY, getUserGo)

const getExperimentPermissionTS = `import * as mlflow from "@baubap/mlflow";

const perm = mlflow.auth.getExperimentPermission({ experimentId: "0", username: "jdoe" });`

const getExperimentPermissionPY = `import baubap_mlflow as mlflow

perm = mlflow.auth.get_experiment_permission(experiment_id="0", username="jdoe")`

const getExperimentPermissionGo = `perm, err := auth.GetExperimentPermission(ctx, &auth.GetExperimentPermissionArgs{
    ExperimentId: "0",
    Username:     "jdoe",
})`

var getExperimentPermissionDesc = "Returns the permission level a user has on an MLflow experiment." + authNote +
	exampleBlock(getExperimentPermissionTS, getExperimentPermissionPY, getExperimentPermissionGo)

const getRegisteredModelPermissionTS = `import * as mlflow from "@baubap/mlflow";

const perm = mlflow.auth.getRegisteredModelPermission({ name: "fraud-detector", username: "jdoe" });`

const getRegisteredModelPermissionPY = `import baubap_mlflow as mlflow

perm = mlflow.auth.get_registered_model_permission(name="fraud-detector", username="jdoe")`

const getRegisteredModelPermissionGo = `perm, err := auth.GetRegisteredModelPermission(ctx, &auth.GetRegisteredModelPermissionArgs{
    Name:     "fraud-detector",
    Username: "jdoe",
})`

var getRegisteredModelPermissionDesc = "Returns the permission level a user has on an MLflow registered model." +
	authNote +
	exampleBlock(getRegisteredModelPermissionTS, getRegisteredModelPermissionPY, getRegisteredModelPermissionGo)
