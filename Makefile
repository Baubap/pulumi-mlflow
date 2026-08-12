PROJECT_NAME := Pulumi MLflow Provider

PACK             := mlflow
PACKDIR          := sdk
PROJECT          := github.com/Baubap/pulumi-mlflow
NODE_MODULE_NAME := @baubap/mlflow

PROVIDER        := pulumi-resource-${PACK}
VERSION_PATH    := provider/version.Version

PULUMI          := pulumi

SCHEMA_FILE     := provider/cmd/pulumi-resource-mlflow/schema.json
export GOPATH   := $(shell go env GOPATH)

WORKING_DIR     := $(shell pwd)
TESTPARALLELISM := 4

# Local & branch builds use this default unless PROVIDER_VERSION is set (CI derives it from the tag).
PROVIDER_VERSION ?= 0.0.1-alpha.0+dev
VERSION_GENERIC   = $(shell pulumictl convert-version --language generic --version "$(PROVIDER_VERSION)")

# MLflow integration test settings (see mlflow-up / test_matrix).
MLFLOW_IMAGE ?= ghcr.io/mlflow/mlflow
MLFLOW_TAG   ?= v3.1.0
MLFLOW_PORT  ?= 5000
export MLFLOW_TRACKING_URI ?= http://localhost:$(MLFLOW_PORT)
# Server config for the ephemeral container — override to test other backends
# (e.g. MLFLOW_SERVER_ARGS="--backend-store-uri postgresql://...").
MLFLOW_SERVER_ARGS ?= --backend-store-uri sqlite:////tmp/mlflow.db --artifacts-destination /tmp/artifacts
# Versions swept by `test_matrix`.
MLFLOW_TAGS  ?= v2.16.2 v3.1.0

export PULUMI_IGNORE_AMBIENT_PLUGINS = true

ensure::
	go mod tidy

# ---- provider binary ---------------------------------------------------------

.PHONY: provider provider_debug
provider: bin/${PROVIDER}

bin/${PROVIDER}:
	go build -o $(WORKING_DIR)/bin/${PROVIDER} -ldflags "-X ${PROJECT}/${VERSION_PATH}=${VERSION_GENERIC}" ${PROJECT}/provider/cmd/${PROVIDER}

provider_debug:
	go build -o $(WORKING_DIR)/bin/${PROVIDER} -gcflags="all=-N -l" -ldflags "-X ${PROJECT}/${VERSION_PATH}=${VERSION_GENERIC}" ${PROJECT}/provider/cmd/${PROVIDER}

# ---- schema ------------------------------------------------------------------

$(SCHEMA_FILE): provider
	$(PULUMI) package get-schema $(WORKING_DIR)/bin/${PROVIDER} | jq 'del(.version)' > $(SCHEMA_FILE)

.PHONY: generate_schema
generate_schema: $(SCHEMA_FILE)

# ---- SDK codegen -------------------------------------------------------------

# codegen generates the schema + the SDKs we publish (nodejs, python, go).
.PHONY: codegen
codegen: $(SCHEMA_FILE) sdk/nodejs sdk/python sdk/go

.PHONY: sdk/%
sdk/%: $(SCHEMA_FILE)
	rm -rf $@
	$(PULUMI) package gen-sdk --language $* $(SCHEMA_FILE) --version "${VERSION_GENERIC}"

sdk/python: $(SCHEMA_FILE)
	rm -rf $@
	$(PULUMI) package gen-sdk --language python $(SCHEMA_FILE) --version "${VERSION_GENERIC}"
	cp README.md ${PACKDIR}/python/

sdk/go: $(SCHEMA_FILE)
	rm -rf $@
	$(PULUMI) package gen-sdk --language go $(SCHEMA_FILE) --version "${VERSION_GENERIC}"
	cd ${PACKDIR}/go/${PACK} && \
		{ test -f go.mod || go mod init ${PROJECT}/${PACKDIR}/go/${PACK}; } && \
		go mod tidy

NPM_DESC := A Pulumi provider to manage MLflow (2.x and 3.x) experiments, model registry and access control via the MLflow REST API.

.PHONY: generate_nodejs generate_python generate_go
generate_nodejs: sdk/nodejs
	@# Enrich the generated package.json so the published npm package looks professional
	@# and a scoped (@baubap) package publishes publicly.
	@jq --arg d "$(NPM_DESC)" \
		'.description=$$d | .author="Baubap" | .bugs={"url":"https://github.com/Baubap/pulumi-mlflow/issues"} | .publishConfig={"access":"public"}' \
		sdk/nodejs/package.json > sdk/nodejs/package.json.tmp && mv sdk/nodejs/package.json.tmp sdk/nodejs/package.json
generate_python: sdk/python
generate_go: sdk/go

.PHONY: build_nodejs build_python build_go
build_nodejs: sdk/nodejs
	cd ${PACKDIR}/nodejs/ && yarn install && yarn run tsc
	cp README.md LICENSE ${PACKDIR}/nodejs/package.json ${PACKDIR}/nodejs/yarn.lock ${PACKDIR}/nodejs/bin/

build_python: sdk/python
	cd ${PACKDIR}/python/ && \
		rm -rf ./bin/ ../python.bin/ && cp -R . ../python.bin && mv ../python.bin ./bin && \
		python3 -m venv venv && \
		./venv/bin/python -m pip install build && \
		cd ./bin && ../venv/bin/python -m build .

build_go: sdk/go

# ---- tests -------------------------------------------------------------------

.PHONY: test_provider
test_provider:
	go test -short -count=1 -cover -timeout 30m -parallel ${TESTPARALLELISM} ./provider/...

# provider/e2e drives the full resource lifecycle (create/update/delete) through
# the provider's in-process server against a REAL MLflow tracking server. No SDKs
# or Pulumi CLI required; only MLFLOW_TRACKING_URI must point at a running server
# (see mlflow-up).
.PHONY: test_e2e
test_e2e:
	go test -v -count=1 -tags=e2e -timeout 45m ./provider/e2e/...

# Alias kept for CI/back-compat.
.PHONY: test_integration
test_integration: test_e2e

.PHONY: install
install:
	cp $(WORKING_DIR)/bin/${PROVIDER} ${GOPATH}/bin

# ---- MLflow server for integration tests (Docker) ----------------------------

.PHONY: mlflow-up mlflow-down test_matrix
mlflow-up:
	docker run -d --name mlflow-it -p $(MLFLOW_PORT):5000 $(MLFLOW_IMAGE):$(MLFLOW_TAG) \
		mlflow server --host 0.0.0.0 --port 5000 $(MLFLOW_SERVER_ARGS)
	@echo "waiting for MLflow ($(MLFLOW_TAG)) at $(MLFLOW_TRACKING_URI) ..."; \
	for i in $$(seq 1 30); do \
		curl -sf $(MLFLOW_TRACKING_URI)/health >/dev/null 2>&1 && echo "ready" && exit 0; \
		sleep 2; \
	done; \
	echo "mlflow did not become ready" && docker logs mlflow-it && exit 1

mlflow-down:
	-docker rm -f mlflow-it

# Sweep the e2e suite across every version in MLFLOW_TAGS (override to test more,
# e.g. `make test_matrix MLFLOW_TAGS="v2.16.2 v3.1.0 latest"`).
.PHONY: test_matrix
test_matrix:
	@for tag in $(MLFLOW_TAGS); do \
		echo "==> MLflow $$tag"; \
		$(MAKE) mlflow-up MLFLOW_TAG=$$tag && $(MAKE) test_e2e; status=$$?; \
		$(MAKE) mlflow-down; \
		test $$status -eq 0 || exit $$status; \
	done

# ---- misc --------------------------------------------------------------------

.PHONY: install-hooks
install-hooks:
	git config core.hooksPath .githooks
	@echo "git hooks enabled (core.hooksPath=.githooks). The pre-commit hook"
	@echo "regenerates schema.json + SDKs when provider Go source changes."

.PHONY: lint
lint:
	golangci-lint --path-prefix provider --config .golangci.yml run --fix

.PHONY: ci-mgmt
ci-mgmt: .ci-mgmt.yaml
	go run github.com/pulumi/ci-mgmt/provider-ci@master generate
