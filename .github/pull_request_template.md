## What / Why

<!-- What does this change do, and why? Link any related issue: Fixes #123 -->

## How

<!-- How is it implemented? Note any REST endpoints, new resources/functions, or behavior changes. -->

## Testing

<!-- How did you verify it? Unit tests, `make test_e2e` against MLflow 2.x/3.x, manual `pulumi up`, etc. -->

## Checklist

- [ ] `go build ./...`, `go vet ./...` and `make test_provider` pass
- [ ] Regenerated the schema (`schema.json`) if resources/functions/config changed
- [ ] Regenerated SDKs if the schema changed (`make generate_nodejs generate_python generate_go`)
- [ ] Added/updated unit tests (and e2e coverage where relevant)
- [ ] Added/updated docs & descriptions (resource `Describe`, `docs/`, examples)
- [ ] Did not commit the `bin/` directory
