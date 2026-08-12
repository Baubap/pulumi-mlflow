---
title: MLflow Installation & Configuration
meta_desc: How to install and configure the MLflow provider for Pulumi, including credentials and configuration options.
layout: package
---

## Installation

The MLflow provider is available as a package in the following Pulumi languages:

- JavaScript/TypeScript: [`@baubap/mlflow`](https://www.npmjs.com/package/@baubap/mlflow) — `npm install @baubap/mlflow`
- Python: [`baubap-mlflow`](https://pypi.org/project/baubap-mlflow/) — `pip install baubap-mlflow`
- Go: `go get github.com/Baubap/pulumi-mlflow/sdk/go/mlflow`

> .NET and Java SDKs are planned for a later release.

The plugin binary is resolved from GitHub Releases via
`pluginDownloadURL github://api.github.com/Baubap/pulumi-mlflow`, so `pulumi plugin install` and SDK
auto-install work without a Registry entry.

## Configuration

The provider needs the URL of your MLflow tracking server and, if the server requires authentication, either a
bearer token or HTTP basic credentials.

| Config key | Description | Environment variable | Secret |
|---|---|---|---|
| `mlflow:trackingUri` | Base URL of the MLflow tracking server, e.g. `https://mlflow.example.com`. | `MLFLOW_TRACKING_URI` | no |
| `mlflow:username` | Username for HTTP basic auth. | `MLFLOW_TRACKING_USERNAME` | no |
| `mlflow:password` | Password for HTTP basic auth. | `MLFLOW_TRACKING_PASSWORD` | **yes** |
| `mlflow:token` | Bearer token (takes precedence over basic auth). | `MLFLOW_TRACKING_TOKEN` | **yes** |
| `mlflow:insecureSkipVerify` | Skip TLS certificate verification (not recommended). | — | no |

### Set configuration

```bash
pulumi config set mlflow:trackingUri https://mlflow.example.com

# Token auth
pulumi config set mlflow:token <token> --secret

# …or basic auth
pulumi config set mlflow:username <user>
pulumi config set mlflow:password <password> --secret
```

### Or use environment variables

```bash
export MLFLOW_TRACKING_URI=https://mlflow.example.com
export MLFLOW_TRACKING_TOKEN=<token>
# or
export MLFLOW_TRACKING_USERNAME=<user>
export MLFLOW_TRACKING_PASSWORD=<password>
```

## Access control (auth) module

The `auth` resources (`User`, `ExperimentPermission`, `RegisteredModelPermission`) require the MLflow server to
be running with the built-in authentication app enabled, e.g.:

```bash
mlflow server --app-name basic-auth
```

If the auth REST endpoints are not available, these resources return an error indicating the `mlflow.server.auth`
app must be enabled.
