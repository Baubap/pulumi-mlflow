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

# MLflow version for the e2e stack (see e2e-up / docker-compose.e2e.yml).
MLFLOW_TAG ?= v2.16.2

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

# provider/e2e drives the full resource lifecycle (create/update/delete) AND
# cross-server ModelVersionPromotion through the provider's in-process server
# against REAL MLflow servers. Bring the stack up with `make e2e-up` first; the
# lifecycle tests run against the dest server, promotion copies source -> dest.
.PHONY: test_e2e
test_e2e:
	MLFLOW_TRACKING_URI=http://localhost:5002 \
	MLFLOW_SOURCE_URI=http://localhost:5001 MLFLOW_DEST_URI=http://localhost:5002 \
	AWS_ACCESS_KEY_ID=minioadmin AWS_SECRET_ACCESS_KEY=minioadmin AWS_REGION=us-east-1 \
	AWS_ENDPOINT_URL_S3=http://localhost:9000 AWS_S3_FORCE_PATH_STYLE=true \
	go test -v -count=1 -tags=e2e -timeout 45m ./provider/e2e/...

# Alias kept for CI/back-compat.
.PHONY: test_integration
test_integration: test_e2e

.PHONY: install
install:
	cp $(WORKING_DIR)/bin/${PROVIDER} ${GOPATH}/bin

# ---- e2e stack (Docker): two MLflow servers + MinIO --------------------------
COMPOSE_E2E := provider/e2e/docker-compose.e2e.yml

.PHONY: e2e-up e2e-down
# MLFLOW_TAG overrides the MLflow version for both servers.
e2e-up:
	MLFLOW_TAG=$(MLFLOW_TAG) docker compose -f $(COMPOSE_E2E) up -d
	@echo "waiting for source (5001) + dest (5002) MLflow ..."; \
	for i in $$(seq 1 60); do \
		curl -sf http://localhost:5001/health >/dev/null 2>&1 && \
		curl -sf http://localhost:5002/health >/dev/null 2>&1 && echo "ready" && exit 0; \
		sleep 2; \
	done; \
	echo "servers did not become ready"; docker compose -f $(COMPOSE_E2E) logs; exit 1

e2e-down:
	-docker compose -f $(COMPOSE_E2E) down -v

# ---- misc --------------------------------------------------------------------

.PHONY: install-hooks
install-hooks:
	lefthook install
	@echo "git hooks installed via lefthook (see lefthook.yml). Requires lefthook:"
	@echo "  brew install lefthook   # or: go install github.com/evilmartians/lefthook@latest"

.PHONY: lint
lint:
	golangci-lint run --fix   # config: .golangci.yml (auto-discovered)

.PHONY: ci-mgmt
ci-mgmt: .ci-mgmt.yaml
	go run github.com/pulumi/ci-mgmt/provider-ci@master generate
