<div align="center">
  <img src="docs/logo.svg" alt="MLflow" height="56" />
  <h1>Pulumi MLflow Provider</h1>
  <p><strong>Manage MLflow experiments, model registry and access control as code.</strong></p>

[![Build](https://github.com/Baubap/pulumi-mlflow/actions/workflows/build.yml/badge.svg)](https://github.com/Baubap/pulumi-mlflow/actions/workflows/build.yml)
[![Integration](https://github.com/Baubap/pulumi-mlflow/actions/workflows/integration.yml/badge.svg)](https://github.com/Baubap/pulumi-mlflow/actions/workflows/integration.yml)
[![npm](https://img.shields.io/npm/v/@baubap/mlflow.svg?label=%40baubap%2Fmlflow)](https://www.npmjs.com/package/@baubap/mlflow)
[![PyPI](https://img.shields.io/pypi/v/baubap-mlflow.svg?label=baubap-mlflow)](https://pypi.org/project/baubap-mlflow/)
[![Go Reference](https://pkg.go.dev/badge/github.com/Baubap/pulumi-mlflow.svg)](https://pkg.go.dev/github.com/Baubap/pulumi-mlflow/sdk/go/mlflow)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](./LICENSE)

</div>

A native [Pulumi](https://www.pulumi.com) provider for [MLflow](https://mlflow.org). It manages the *declarative*
parts of an MLflow estate — experiments, the model registry (registered models, versions, aliases) and access
control (users and permissions) — through the MLflow REST API, so they can be reviewed and rolled out like the rest
of your infrastructure.

- 🧪 **Experiments** — declare experiments, artifact locations and tags.
- 📦 **Model Registry** — registered models, immutable model versions, and mutable deployment **aliases**.
- 🔐 **Access control** — users and per-experiment / per-model permissions (with the `mlflow.server.auth` app).
- 🔁 **MLflow 2.x and 3.x** — one code path; the provider detects the server version at runtime.
- 🌐 **Multi-language** — TypeScript/JavaScript, Python, and Go (.NET and Java planned).

> **Status — `v0.1.0`.** First release covering the stable tracking + registry + auth surface. The API may still
> change on minor releases while it stabilizes. MLflow 3.x-only modules (LoggedModel, Prompt, AI Gateway,
> Webhooks) are on the [roadmap](#roadmap).

## Contents

- [Installation](#installation)
- [Configuration](#configuration)
- [Quick start](#quick-start)
- [Resources & functions](#resources--functions)
- [MLflow version support](#mlflow-version-support)
- [Development](#development)
- [Testing](#testing)
- [Publishing](#publishing)
- [Roadmap](#roadmap)
- [License](#license)

## Installation

```bash
# TypeScript / JavaScript
npm install @baubap/mlflow

# Python
pip install baubap-mlflow

# Go
go get github.com/Baubap/pulumi-mlflow/sdk/go/mlflow
```

The plugin binary is resolved from GitHub Releases via `pluginDownloadURL github://api.github.com/Baubap/pulumi-mlflow`,
so `pulumi plugin install` and SDK auto-install work out of the box.

## Configuration

| Config key | Env var | Secret | Description |
|---|---|:---:|---|
| `mlflow:trackingUri` | `MLFLOW_TRACKING_URI` | | Base URL of the tracking server. |
| `mlflow:token` | `MLFLOW_TRACKING_TOKEN` | ✅ | Bearer token (takes precedence over basic auth). |
| `mlflow:username` | `MLFLOW_TRACKING_USERNAME` | | Username for HTTP basic auth. |
| `mlflow:password` | `MLFLOW_TRACKING_PASSWORD` | ✅ | Password for HTTP basic auth. |
| `mlflow:insecureSkipVerify` | | | Skip TLS verification (not recommended). |

```bash
pulumi config set mlflow:trackingUri https://mlflow.internal.example.com
pulumi config set mlflow:token <token> --secret
```

## Quick start

```typescript
import * as mlflow from "@baubap/mlflow";

const experiment = new mlflow.Experiment("demo", {
    name: "fraud-detection",
    tags: { team: "foundations" },
});

const model = new mlflow.registry.RegisteredModel("fraud", {
    name: "fraud-detector",
    description: "Gradient-boosted fraud model",
});

const v1 = new mlflow.registry.ModelVersion("fraud-v1", {
    name: model.name,
    source: "s3://models/fraud/1",
});

// Point deployments at a mutable alias instead of a deprecated stage.
new mlflow.registry.RegisteredModelAlias("champion", {
    modelName: model.name,
    alias: "champion",
    version: v1.version,
});
```

```python
import baubap_mlflow as mlflow

experiment = mlflow.Experiment("demo", name="fraud-detection", tags={"team": "foundations"})
model = mlflow.registry.RegisteredModel("fraud", name="fraud-detector",
                                        description="Gradient-boosted fraud model")
v1 = mlflow.registry.ModelVersion("fraud-v1", name=model.name, source="s3://models/fraud/1")
mlflow.registry.RegisteredModelAlias("champion", model_name=model.name, alias="champion", version=v1.version)
```

## Resources & functions

| Module | Resources | Functions (data sources) |
|---|---|---|
| `index` (tracking) | `Experiment` | `getExperiment`, `getExperimentByName`, `searchExperiments` |
| `registry` | `RegisteredModel`, `ModelVersion`, `RegisteredModelAlias`, `RegisteredModelTag` | `getRegisteredModel`, `searchRegisteredModels`, `getLatestVersions`, `getModelVersion`, `searchModelVersions`, `getModelVersionByAlias`, `getModelVersionDownloadUri` |
| `auth`¹ | `User`, `ExperimentPermission`, `RegisteredModelPermission` | `getUser`, `getExperimentPermission`, `getRegisteredModelPermission` |

¹ The `auth` resources require the tracking server to run with the `mlflow.server.auth` app enabled.

Every resource supports `pulumi import`. See the per-resource **Import** sections in the
[registry docs](https://www.pulumi.com/registry/packages/mlflow/) once published.

## MLflow version support

The declarative REST surface is identical across MLflow **2.x and 3.x**, so the same program works against either.
The provider version is **independent** of the MLflow version — MLflow compatibility is documented and handled at
runtime (via `GET /version`), not encoded in the provider's semver. Model-version *stages* are deprecated in
MLflow 3; use `RegisteredModelAlias` instead.

## Development

See **[CONTRIBUTING.md](./CONTRIBUTING.md)** for the full development workflow, project layout, conventions, and how
to add resources, functions, and docs.

```bash
make provider          # build the plugin binary into ./bin
make generate_schema   # regenerate provider/cmd/pulumi-resource-mlflow/schema.json
make generate_nodejs generate_python generate_go   # generate the SDKs into ./sdk
```

## Testing

```bash
make test_provider     # hermetic unit tests (no server needed)

# End-to-end against a real MLflow server (Docker):
make mlflow-up && make test_e2e && make mlflow-down

# Sweep the e2e suite across MLflow versions:
make test_matrix MLFLOW_TAGS="v2.16.2 v3.1.0"
```

The e2e suite (`provider/e2e`, build tag `e2e`) drives the full resource lifecycle through the provider's
in-process server against real MLflow 2.x/3.x servers — see [`integration.yml`](.github/workflows/integration.yml).
It is parametrizable over multiple servers via `MLFLOW_TRACKING_URIS`.

## Publishing

Tagging `vX.Y.Z` triggers [`release.yml`](.github/workflows/release.yml): GoReleaser builds the multi-arch plugin
and creates the GitHub Release, then the SDKs are published to npm and PyPI (Go via a module tag). Listing on the
public Pulumi Registry is a one-time PR to `pulumi/registry` — see [docs/registry-submission.md](docs/registry-submission.md).

## Roadmap

- **v0.2.0+** — MLflow 3.x-only modules behind version gating: `Webhook`, LoggedModel/Scorer/EvaluationDataset,
  Prompt/PromptAlias, and the AI Gateway (endpoints, model definitions, secrets, budgets, guardrails).
- Additional language SDKs: .NET and Java.

## License

[Apache-2.0](./LICENSE).
