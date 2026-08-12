# Publishing to the public Pulumi Registry

Getting `pulumi-mlflow` listed at `pulumi.com/registry/packages/mlflow` is a one-time PR against
[`pulumi/registry`](https://github.com/pulumi/registry), separate from this repository. The listing is generated
from this repo's committed `schema.json` and `docs/`.

## Prerequisites

1. A GitHub Release tagged `vX.Y.Z` exists in `Baubap/pulumi-mlflow` (the schema/API docs are pulled from it).
2. The per-language SDKs are published (npm, PyPI, Go module tag). See `.github/workflows/release.yml`.
3. `provider/cmd/pulumi-resource-mlflow/schema.json` is committed and up to date (`make generate_schema`).
4. `docs/logo.svg` is the final, whitespace-trimmed wordmark (replace the placeholder).

## Submission steps

1. Fork and clone `pulumi/registry`.
2. Add one entry to `community-packages/package-list.json`:
   ```json
   {
     "repoSlug": "Baubap/pulumi-mlflow",
     "schemaFile": "provider/cmd/pulumi-resource-mlflow/schema.json"
   }
   ```
3. Copy `docs/_index.md` and `docs/installation-configuration.md` into
   `themes/default/content/registry/packages/mlflow/`.
4. Open the PR. Automated checks post a fact-sheet; fix issues in this repo and comment `/check` to re-run.
5. A Pulumi maintainer may comment `/preview`, then approves and merges. On merge, CI publishes the listing and
   auto-generated API docs. Nothing merges automatically.

## Schema metadata (already wired into the provider builder)

`displayName=MLflow`, `publisher=Baubap`, `repository`, `homepage`, `license=Apache-2.0`, `logoUrl`,
`pluginDownloadURL=github://api.github.com/Baubap/pulumi-mlflow`, and
`keywords=[…, category/infrastructure, kind/native]` — see `provider/provider.go`.
