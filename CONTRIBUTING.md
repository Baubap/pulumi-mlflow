# Contributing to pulumi-mlflow

Thanks for helping improve the Pulumi MLflow provider! This guide covers the repository layout, the local
development loop, how to add resources/functions/docs, testing, and the release flow.

## Prerequisites

| Tool | Version | Notes |
|---|---|---|
| Go | 1.26+ | the provider is written in Go |
| Pulumi CLI | 3.x | `pulumi package get-schema` / `gen-sdk` |
| pulumictl | latest | version normalization (`brew install pulumictl`) |
| jq | any | schema/SDK metadata munging |
| Node.js + npm | 20+ | build/test the TypeScript SDK |
| Python | 3.9+ | build/test the Python SDK |
| Docker | any | end-to-end tests spin up a real MLflow server |

## Repository layout

```
provider/
  client/     # shared MLflow REST client, Config, auth, error handling, version detection, tag helpers
  version/    # var Version string (injected at build via -ldflags)
  tracking/   # module "index": Experiment (+ docs.go) + data-source functions
  registry/   # module "registry": RegisteredModel, ModelVersion, RegisteredModelAlias (+ docs.go)
  auth/       # module "auth": User, ExperimentPermission, RegisteredModelPermission (+ descriptions.go)
  e2e/        # build-tagged (e2e) lifecycle tests against a real server
  provider.go # infer.NewProviderBuilder(): metadata + aggregates modules
  modules.go  # wires tracking/registry/auth Resources()+Functions() together
  cmd/pulumi-resource-mlflow/  # main.go + committed schema.json
docs/         # _index.md, installation-configuration.md, how-to/, logo.svg
examples/     # python/ + typescript/ example programs
sdk/          # generated SDKs (nodejs, python, go) — regenerated, do not hand-edit
.github/workflows/  # build.yml (PR), integration.yml (e2e matrix), release.yml (tag)
```

The provider uses [`pulumi-go-provider`](https://github.com/pulumi/pulumi-go-provider)'s `infer` framework. Each
domain lives in its own Go package; the Pulumi **module** name (`index`/`registry`/`auth`) is set explicitly in
each resource's `Annotate` via `a.SetToken("<module>", "<Name>")`, so code layout and module names are decoupled.

## Development loop

```bash
# 1. Build the plugin binary (into ./bin)
go build -o bin/pulumi-resource-mlflow \
  -ldflags "-X github.com/Baubap/pulumi-mlflow/provider/version.Version=0.0.1-dev" \
  ./provider/cmd/pulumi-resource-mlflow

# 2. Regenerate the committed schema (source of truth for SDKs + Registry docs)
pulumi package get-schema ./bin/pulumi-resource-mlflow | jq 'del(.version)' \
  > provider/cmd/pulumi-resource-mlflow/schema.json

# 3. Regenerate SDKs if the schema changed
make generate_nodejs generate_python generate_go PROVIDER_VERSION=0.1.0

# 4. Test
make test_provider          # hermetic unit tests
```

> ⚠️ **Gotcha:** `make provider` declares its output binary with no source prerequisites, so it will **not**
> rebuild after you change Go code. Run the explicit `go build -o bin/...` above (or `rm bin/pulumi-resource-mlflow`
> first) before regenerating the schema, or you'll generate from a stale binary.

`provider/cmd/pulumi-resource-mlflow/schema.json` and the generated `sdk/` are **committed**. CI (`build.yml`)
regenerates the schema and fails on drift, so they must stay in sync with the Go code.

Run **`make install-hooks`** once (needs [lefthook](https://lefthook.dev) — `brew install lefthook`, or
`go install github.com/evilmartians/lefthook@latest`). It installs a lefthook pre-commit hook (see `lefthook.yml`)
that regenerates the schema and SDKs and stages them whenever you commit a change to `provider/**.go`, so they
never drift. The committed SDKs carry a fixed `0.0.0` placeholder version; the real version is injected from the
git tag at release.

## Adding a resource

1. Create `provider/<module>/<resource>.go` with an input struct, a state struct (embedding the inputs), and the
   `infer` CRUD methods:

   ```go
   type Widget struct{}

   type WidgetArgs struct {
       Name string  `pulumi:"name" provider:"replaceOnChanges"` // immutable → replace
       Size *int    `pulumi:"size,optional"`
       Token string `pulumi:"token,optional" provider:"secret"` // secret input
   }
   type WidgetState struct {
       WidgetArgs
       WidgetId string `pulumi:"widgetId"`
   }

   func (Widget) Create(ctx context.Context, req infer.CreateRequest[WidgetArgs]) (infer.CreateResponse[WidgetState], error) { ... }
   func (Widget) Read(ctx context.Context, req infer.ReadRequest[WidgetArgs, WidgetState]) (infer.ReadResponse[WidgetArgs, WidgetState], error) { ... }
   func (Widget) Update(...) ... // only if the API supports in-place updates
   func (Widget) Delete(...) ...

   func (w *Widget) Annotate(a infer.Annotator) {
       a.SetToken("index", "Widget")
       a.Describe(w, widgetDesc) // rich description, see "Docs" below
   }
   ```

2. Access the configured client with `infer.GetConfig[client.Config](ctx).Client()` and call the REST API with
   `c.Do(ctx, method, "relative/path", query, body, &out)` — `Do` prepends `/api/2.0/mlflow/`; **GET/DELETE** pass
   params via `query url.Values`, **POST/PATCH** via `body`.
3. Register it in `provider/<module>/<module>.go` (`Resources()` / `Functions()` slices).

### Conventions

- **Immutable fields** (can't be changed in place) → `provider:"replaceOnChanges"` (obeyed by the default diff; do
  not implement a custom `Diff`).
- **Secrets** → `provider:"secret"`; never read secret values back in `Read`.
- **Tags** → model as `map[string]string` and sync in `Update` using `client.DiffTags` + the entity's
  `set-tag`/`delete-tag` endpoints.
- **Not-found** → in `Read`, return an empty `ID` when `client.IsNotFound(err)` so Pulumi detects deletion/drift.
- **Composite IDs** — use `<parent>/<child>` (e.g. ModelVersion `name/version`) and parse in `Read`/`Delete`.
- Every input/output field gets a doc-comment via `a.Describe(&field, "...")` — these become the Registry API docs.

## Docs (Registry-grade)

Resource/function descriptions come **only** from `a.Describe(...)` — `infer` does not read Go doc-comments. To get
per-resource examples on the Registry, embed markdown with the `{{% examples %}}` / `{{< chooser >}}` shortcodes and
a `## Import` section in the description string.

> ⚠️ **Backtick gotcha:** example markdown contains ```` ``` ```` code fences. Go **raw strings can't contain
> backticks**. Pick one of the patterns already used in the repo: double-quoted concatenation (`tracking/docs.go`),
> helper funcs that add fences (`auth/descriptions.go`), or the `md()` sentinel that turns `§` into a backtick
> (`registry/docs.go`).

Overview and install pages live in `docs/_index.md` and `docs/installation-configuration.md` (with YAML
front-matter). How-to guides go in `docs/how-to/`.

## Testing

- **Unit (hermetic):** each module has `_test.go` that stands up an `httptest` fake MLflow, points the provider at
  it, and runs `integration.LifeCycleTest`. Note that `LifeCycleTest` does **not** call `Configure` — the test must
  call `server.Configure(...)` with the tracking URI. Run with `make test_provider`.
- **End-to-end (real server):** `provider/e2e` (build tag `e2e`) drives the full lifecycle against a real MLflow
  server via the in-process provider (no SDKs needed).

  ```bash
  make mlflow-up && make test_e2e && make mlflow-down     # single version
  make test_matrix MLFLOW_TAGS="v2.16.2 v3.1.0"           # sweep versions
  # Point at existing servers instead of Docker:
  MLFLOW_TRACKING_URIS="https://a,https://b" make test_e2e
  ```

  > On macOS, port 5000 is often taken by AirPlay — use `MLFLOW_PORT=5050`. Model-version `source` must be a
  > non-local URI (e.g. `s3://...`) unless a `run_id` is supplied.

- **Lint:** `make lint` (golangci-lint).

## Versioning & releases

- The provider's semver is **independent** of MLflow's version. MLflow 2.x/3.x compatibility is documented and
  handled at runtime, never encoded in the tag.
- Tagging `vX.Y.Z` (non-prerelease) triggers `release.yml`: GoReleaser builds the plugin + GitHub Release, then the
  SDKs publish to npm/PyPI and the Go module tag. Pre-release tags (`vX.Y.Z-*`) are excluded.
- Breaking schema changes are surfaced by `pulumi/schema-tools` in CI and require a MAJOR bump (once past `v1.0.0`).

## Pull requests

1. Branch off `main`.
2. Make your change, regenerate the schema (and SDKs if the schema changed), and add/adjust tests.
3. Ensure `go build ./...`, `go vet ./...`, `make test_provider`, and the schema-drift gate pass.
4. Open a PR; do not commit the `bin/` directory (it's gitignored).

## Filing issues & triage

- **Bugs** and **feature requests** use the structured forms under
  [`.github/ISSUE_TEMPLATE/`](.github/ISSUE_TEMPLATE) — they ask for the provider version, the **MLflow server
  version (2.x/3.x)**, language SDK, and a reproduction, which is what we need to act on a report.
- **Usage questions** go to [Discussions](https://github.com/Baubap/pulumi-mlflow/discussions), not issues.
- **Security** vulnerabilities are reported privately — see [SECURITY.md](./SECURITY.md).

New issues land with `needs-triage`. During triage a maintainer confirms/reproduces, replaces `needs-triage` with a
`kind/*` (bug/enhancement) and an `area/*` label (`tracking`, `registry`, `auth`, `provider`, or a 3.x module), and
sets priority. Good starter issues are tagged `good-first-issue`.

> Labels for this repo are provisioned through the Baubap **infrastructure** (Pulumi) repo, not created by hand —
> keep the `kind/*` / `area/*` / `needs-triage` / `good-first-issue` taxonomy in sync there.

## Adding a new module (roadmap work)

Milestone 2 adds MLflow 3.x-only modules (`events`, `genai`, `gateway`). To add one: create `provider/<module>/`
with its resources + `Resources()`/`Functions()`, add the package to `provider/modules.go`, gate 3.x-only behavior
on `client.ServerMajorVersion(ctx)`, and add docs + e2e coverage.
