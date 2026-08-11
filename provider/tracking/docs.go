package tracking

// Rich schema descriptions for the tracking module. These render on each
// resource/function page in the Pulumi Registry, so they embed copy-paste
// examples via the {{% examples %}} / {{< chooser >}} shortcodes. They are
// built with double-quoted concatenation because Go raw strings cannot contain
// the ``` code-fence backticks.

const experimentDesc = "" +
	"Manages an MLflow **experiment** — a named container that groups runs and their artifacts.\n\n" +
	"Backed by the MLflow [experiments REST API](https://mlflow.org/docs/latest/api_reference/rest-api.html#create-experiment). " +
	"Works against MLflow tracking servers 2.x and 3.x.\n\n" +
	"> **Note:** MLflow exposes no `delete-experiment-tag` endpoint, so removing a tag in place is not supported; " +
	"remove it out of band or recreate the experiment. `artifactLocation` is set once at creation — changing it replaces the experiment.\n\n" +
	"{{% examples %}}\n## Example Usage\n{{% example %}}\n" +
	"{{< chooser language \"typescript,python,go\" >}}\n" +
	"{{% choosable language typescript %}}\n" +
	"```typescript\nimport * as mlflow from \"@baubap/mlflow\";\n\n" +
	"const experiment = new mlflow.Experiment(\"demo\", {\n    name: \"fraud-detection\",\n    tags: { team: \"foundations\" },\n});\n```\n" +
	"{{% /choosable %}}\n" +
	"{{% choosable language python %}}\n" +
	"```python\nimport baubap_mlflow as mlflow\n\n" +
	"experiment = mlflow.Experiment(\"demo\",\n    name=\"fraud-detection\",\n    tags={\"team\": \"foundations\"})\n```\n" +
	"{{% /choosable %}}\n" +
	"{{% choosable language go %}}\n" +
	"```go\n_, err := mlflow.NewExperiment(ctx, \"demo\", &mlflow.ExperimentArgs{\n" +
	"    Name: pulumi.String(\"fraud-detection\"),\n    Tags: pulumi.StringMap{\"team\": pulumi.String(\"foundations\")},\n})\n```\n" +
	"{{% /choosable %}}\n" +
	"{{< /chooser >}}\n{{% /example %}}\n{{% /examples %}}\n\n" +
	"## Import\n\n" +
	"An existing experiment can be imported by its server-assigned ID:\n\n" +
	"```sh\npulumi import mlflow:index:Experiment demo <experiment_id>\n```"

const getExperimentDesc = "" +
	"Looks up an existing MLflow experiment by its server-assigned ID. " +
	"Wraps [`experiments/get`](https://mlflow.org/docs/latest/api_reference/rest-api.html#get-experiment).\n\n" +
	"{{% examples %}}\n## Example Usage\n{{% example %}}\n" +
	"{{< chooser language \"typescript,python,go\" >}}\n" +
	"{{% choosable language typescript %}}\n" +
	"```typescript\nimport * as mlflow from \"@baubap/mlflow\";\n\nconst exp = await mlflow.getExperiment({ experimentId: \"123\" });\n```\n" +
	"{{% /choosable %}}\n" +
	"{{% choosable language python %}}\n" +
	"```python\nimport baubap_mlflow as mlflow\n\nexp = mlflow.get_experiment(experiment_id=\"123\")\n```\n" +
	"{{% /choosable %}}\n" +
	"{{% choosable language go %}}\n" +
	"```go\nexp, err := mlflow.GetExperiment(ctx, &mlflow.GetExperimentArgs{ExperimentId: \"123\"}, nil)\n```\n" +
	"{{% /choosable %}}\n" +
	"{{< /chooser >}}\n{{% /example %}}\n{{% /examples %}}"

const getExperimentByNameDesc = "" +
	"Looks up an existing MLflow experiment by its unique name. " +
	"Wraps [`experiments/get-by-name`](https://mlflow.org/docs/latest/api_reference/rest-api.html#get-experiment-by-name).\n\n" +
	"{{% examples %}}\n## Example Usage\n{{% example %}}\n" +
	"{{< chooser language \"typescript,python,go\" >}}\n" +
	"{{% choosable language typescript %}}\n" +
	"```typescript\nimport * as mlflow from \"@baubap/mlflow\";\n\nconst exp = await mlflow.getExperimentByName({ name: \"fraud-detection\" });\n```\n" +
	"{{% /choosable %}}\n" +
	"{{% choosable language python %}}\n" +
	"```python\nimport baubap_mlflow as mlflow\n\nexp = mlflow.get_experiment_by_name(name=\"fraud-detection\")\n```\n" +
	"{{% /choosable %}}\n" +
	"{{% choosable language go %}}\n" +
	"```go\nexp, err := mlflow.GetExperimentByName(ctx, &mlflow.GetExperimentByNameArgs{Name: \"fraud-detection\"}, nil)\n```\n" +
	"{{% /choosable %}}\n" +
	"{{< /chooser >}}\n{{% /example %}}\n{{% /examples %}}"

const searchExperimentsDesc = "" +
	"Searches MLflow experiments with an optional filter expression. " +
	"Wraps [`experiments/search`](https://mlflow.org/docs/latest/api_reference/rest-api.html#search-experiments).\n\n" +
	"{{% examples %}}\n## Example Usage\n{{% example %}}\n" +
	"{{< chooser language \"typescript,python,go\" >}}\n" +
	"{{% choosable language typescript %}}\n" +
	"```typescript\nimport * as mlflow from \"@baubap/mlflow\";\n\nconst results = await mlflow.searchExperiments({ filter: \"tags.team = 'foundations'\" });\n```\n" +
	"{{% /choosable %}}\n" +
	"{{% choosable language python %}}\n" +
	"```python\nimport baubap_mlflow as mlflow\n\nresults = mlflow.search_experiments(filter=\"tags.team = 'foundations'\")\n```\n" +
	"{{% /choosable %}}\n" +
	"{{% choosable language go %}}\n" +
	"```go\nresults, err := mlflow.SearchExperiments(ctx, &mlflow.SearchExperimentsArgs{Filter: pulumi.StringRef(\"tags.team = 'foundations'\")}, nil)\n```\n" +
	"{{% /choosable %}}\n" +
	"{{< /chooser >}}\n{{% /example %}}\n{{% /examples %}}"
