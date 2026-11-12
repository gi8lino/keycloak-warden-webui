.DEFAULT_GOAL := help

## Tool Versions

# renovate: datasource=github-releases depName=golangci/golangci-lint
GOLANGCI_LINT_VERSION ?= v2.15.0

# renovate: datasource=github-releases depName=gi8lino/dev-tools
DEV_TOOLS_VERSION ?= v0.9.0

# renovate: datasource=github-releases depName=kumbuka-me/cli
KUMBUKA_CLI_VERSION ?= v0.17.0
KUMBUKA_CLI := bin/.kumbuka/$(KUMBUKA_CLI_VERSION)/kumbuka-cli
KUMBUKA_CLI_ASSET ?= kumbuka-cli_{version}_{os}_{arch}.tar.gz

## End-to-end Compose Configuration

ifneq ($(filter e2e-oidc-%,$(MAKECMDGOALS)),)
E2E_PROJECT ?= warden-webui-oidc
E2E_WARDEN_PORT ?= 18181
E2E_WARDEN_MANAGEMENT_PORT ?= 18182
E2E_HOOKGLASS_PORT ?= 18190
E2E_WEBUI_PORT ?= 18192
E2E_WEBUI_MANAGEMENT_PORT ?= 18193
export E2E_KEYCLOAK_PORT ?= 18180
E2E_OVERLAY := -f deploy/compose.oidc.yaml
endif

E2E_PROJECT ?= warden-webui-e2e
E2E_COMPOSE_FILE ?= deploy/compose.yaml
E2E_WARDEN_DIR ?= ../warden
E2E_WARDEN_CONTEXT := $(abspath $(E2E_WARDEN_DIR))
E2E_WARDEN_PORT ?= 18081
E2E_WARDEN_MANAGEMENT_PORT ?= 18082
E2E_HOOKGLASS_PORT ?= 18090
E2E_WEBUI_PORT ?= 18092
E2E_WEBUI_MANAGEMENT_PORT ?= 18093
E2E_COMPOSE := docker compose -p $(E2E_PROJECT) -f $(E2E_COMPOSE_FILE) $(E2E_OVERLAY)

export E2E_WARDEN_CONTEXT
export E2E_WARDEN_PORT E2E_WARDEN_MANAGEMENT_PORT E2E_HOOKGLASS_PORT
export E2E_WEBUI_PORT E2E_WEBUI_MANAGEMENT_PORT

## Shared Development Tools

include bin/dev-tools.mk
include $(call dev-tools-module,tag)
include $(call dev-tools-module,port)
include $(call dev-tools-module,browser)
include $(call dev-tools-module,help)

## Project-local Tools

GOLANGCI_LINT := bin/.golangci-lint/$(GOLANGCI_LINT_VERSION)/golangci-lint
FAVICON_GENERATE := $(DEV_TOOLS_BIN)/favicon-generate
SVG_TO_PNG := $(DEV_TOOLS_BIN)/svg-to-png

## Build Configuration

BINARY ?= build/warden-webui
COMMAND ?= ./cmd/warden-webui
BUILD_VERSION ?= dev
LDFLAGS ?= -s -w -X main.Version=$(BUILD_VERSION)

## Local Development

WEBUI_ASSIGNED_PORT ?= $(call dev-port,webui)
WEBUI_MANAGEMENT_ASSIGNED_PORT ?= $(call dev-port,management)

LISTEN_ADDRESS ?= 127.0.0.1:$(WEBUI_ASSIGNED_PORT)
MANAGEMENT_LISTEN_ADDRESS ?= 127.0.0.1:$(WEBUI_MANAGEMENT_ASSIGNED_PORT)
PUBLIC_URL ?= http://127.0.0.1:$(WEBUI_ASSIGNED_PORT)
DATABASE ?= warden-webui.db
RUN_ARGS ?=

## Documentation Configuration

SITE_CONFIG ?= docs/site.toml
SITE_PORT = $(call dev-port,site)
SITE_URL = http://127.0.0.1:$(SITE_PORT)/
BUILD_ARGS ?=

## Asset Configuration

LOGO_SOURCE ?= web/static/assets/warden-webui.svg
LOGO_PNG ?= web/static/assets/warden-webui.png
LOGO_PNG_WIDTH ?= 1200
FAVICON_SOURCE ?= $(LOGO_SOURCE)
FAVICON_DIR ?= web/static/assets
FAVICON_SIZES ?= 16x16 32x32

##@ Development

.PHONY: ports
ports: $(DEV_PORT) ## Print and persist the assigned local HTTP ports.
	@$(DEV_PORT) webui --port "$(WEBUI_ASSIGNED_PORT)" > /dev/null
	@$(DEV_PORT) management --port "$(WEBUI_MANAGEMENT_ASSIGNED_PORT)" > /dev/null
	@echo "Web UI:     $(PUBLIC_URL)/"
	@echo "Management: http://$(MANAGEMENT_LISTEN_ADDRESS)/healthz"

.PHONY: ports-reset
ports-reset: $(DEV_PORT) ## Clear saved ports after stopping local services.
	$(call run-tool,$(DEV_PORT),--reset)

.PHONY: download
download: dev-tools golangci-lint ## Download Go modules and pinned development tools.
	go mod download

.PHONY: build
build: ## Build the Warden Web UI binary.
	@mkdir -p "$(dir $(BINARY))"
	CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o "$(BINARY)" "$(COMMAND)"

.PHONY: serve
serve: ports ## Run Warden Web UI with assigned ports.
	go run "$(COMMAND)" \
		--listen-address="$(LISTEN_ADDRESS)" \
		--management-listen-address="$(MANAGEMENT_LISTEN_ADDRESS)" \
		--public-url="$(PUBLIC_URL)" \
		--database="$(DATABASE)" \
		$(RUN_ARGS)

.PHONY: run
run: ## Run Warden Web UI locally.
	$(MAKE) serve RUN_ARGS="$(RUN_ARGS)"

.PHONY: clean
clean: ## Remove built binaries and coverage reports.
	rm -rf build
	rm -f coverage.out coverage.html

##@ Assets

$(FAVICON_GENERATE): | $(DEV_TOOLS_BIN)
	$(call download-dev-tool,favicon-generate,$@)

$(SVG_TO_PNG): | $(DEV_TOOLS_BIN)
	$(call download-dev-tool,svg-to-png,$@)

.PHONY: logo-png
logo-png: $(SVG_TO_PNG) $(LOGO_SOURCE) ## Generate a PNG version of the Web UI logo.
	$(call run-tool,$(SVG_TO_PNG),--width "$(LOGO_PNG_WIDTH)" "$(LOGO_SOURCE)" "$(LOGO_PNG)")

.PHONY: favicon
favicon: $(FAVICON_GENERATE) $(FAVICON_SOURCE) ## Generate Web UI favicons from the SVG logo.
	$(call run-tool,$(FAVICON_GENERATE),--apple-touch "$(FAVICON_SOURCE)" "$(FAVICON_DIR)" $(FAVICON_SIZES))

##@ Documentation

.PHONY: build-docs
build-docs: cli ## Build the production Kumbuka documentation site.
	$(call run-tool,$(KUMBUKA_CLI),build --config "$(SITE_CONFIG)" $(BUILD_ARGS))

.PHONY: serve-docs
serve-docs: $(DEV_PORT) $(OPEN_BROWSER) cli ## Serve, watch, and open the documentation locally.
	@(until curl --fail --silent --output /dev/null "$(SITE_URL)"; do sleep 0.2; done; \
		$(OPEN_BROWSER) "$(SITE_URL)") & \
	browser_pid=$$!; \
	trap 'kill "$$browser_pid" 2>/dev/null || true' EXIT; \
	$(KUMBUKA_CLI) serve \
		--config "$(SITE_CONFIG)" \
		--site-url "$(SITE_URL)" \
		--listen-address "127.0.0.1:$(SITE_PORT)"

##@ End-to-end Compose

.PHONY: e2e-up
e2e-up: ## Build and run the isolated Warden, Hookglass, and Web UI stack.
	@test -f "$(E2E_WARDEN_CONTEXT)/go.mod" && test -f "$(E2E_WARDEN_CONTEXT)/Dockerfile" || \
		{ echo "Warden source not found at $(E2E_WARDEN_CONTEXT). Set E2E_WARDEN_DIR=../keycloak-warden (or your checkout path)." >&2; exit 1; }
	$(E2E_COMPOSE) up --build --detach --remove-orphans
	@set -eu; \
	wait_ready() { \
		label="$$1"; url="$$2"; attempt=0; \
		until curl --fail --silent --output /dev/null --max-time 2 "$$url"; do \
			attempt=$$((attempt + 1)); \
			if [ "$$attempt" -ge 60 ]; then \
				printf 'Timed out waiting for %s (%s)\n' "$$label" "$$url" >&2; \
				$(E2E_COMPOSE) ps >&2; \
				$(E2E_COMPOSE) logs --tail=80 >&2; \
				exit 1; \
			fi; \
			sleep 2; \
		done; \
		printf '%s ready\n' "$$label"; \
	}; \
	wait_ready Warden "http://127.0.0.1:$(E2E_WARDEN_MANAGEMENT_PORT)/readyz"; \
	wait_ready Hookglass "http://127.0.0.1:$(E2E_HOOKGLASS_PORT)/healthz"; \
	wait_ready 'Warden Web UI' "http://127.0.0.1:$(E2E_WEBUI_MANAGEMENT_PORT)/readyz"
	@printf '\nWeb UI:    http://127.0.0.1:$(E2E_WEBUI_PORT)/\n'
	@printf 'Setup:     http://127.0.0.1:$(E2E_WEBUI_PORT)/setup\n'
	@printf 'Token:     development-setup-token-0123456789abcdef (demo only)\n'
	@printf 'Hookglass: http://127.0.0.1:$(E2E_HOOKGLASS_PORT)/header (webhook receiver)\n'
	@printf 'Warden:    http://127.0.0.1:$(E2E_WARDEN_PORT)/\n'
	@printf 'Logs:      make $(if $(E2E_OVERLAY),e2e-oidc-logs,e2e-logs)\n'
	@printf 'Stop:      make $(if $(E2E_OVERLAY),e2e-oidc-down,e2e-down)\n'

.PHONY: e2e-logs
e2e-logs: ## Follow the local end-to-end stack logs.
	$(E2E_COMPOSE) logs --follow --tail=100

.PHONY: e2e-down
e2e-down: ## Stop the stack without deleting SQLite data.
	$(E2E_COMPOSE) down --remove-orphans

.PHONY: e2e-reset
e2e-reset: ## Stop the stack and delete its dedicated SQLite volumes.
	$(E2E_COMPOSE) down --volumes --remove-orphans

.PHONY: e2e-oidc-up
e2e-oidc-up: e2e-up ## Start the local Keycloak OIDC end-to-end stack.
	@printf 'Keycloak login: operator / test-provider-password (demo only)\n'

.PHONY: e2e-oidc-logs
e2e-oidc-logs: e2e-logs ## Follow the OIDC end-to-end stack logs.

.PHONY: e2e-oidc-down
e2e-oidc-down: e2e-down ## Stop the OIDC end-to-end stack.

.PHONY: e2e-oidc-reset
e2e-oidc-reset: e2e-reset ## Delete the OIDC end-to-end stack and SQLite volumes.

##@ Testing

.PHONY: test
test: ## Run Go tests.
	go test -count=1 -timeout=3m ./...

.PHONY: test-race
test-race: ## Run Go tests with the race detector.
	go test -race -count=1 -timeout=3m ./...

.PHONY: test-auth-e2e
test-auth-e2e: build ## Exercise local, trusted-proxy, and Keycloak browser login.
	E2E_WEBUI_BINARY="$(abspath $(BINARY))" npx --no-install playwright test test/auth --workers=3 --output=build/auth-test-results

.PHONY: cover
cover: ## Generate and view Go test coverage.
	go test -coverprofile=coverage.out -covermode=atomic -count=1 -timeout=3m ./...
	go tool cover -html=coverage.out

##@ Formatting

.PHONY: fmt
fmt: ## Format Go source code.
	go fmt ./...

.PHONY: check-format
check-format: ## Verify Go source formatting and whitespace.
	@set -eu; unformatted="$$(gofmt -l cmd internal web)"; \
	if [ -n "$$unformatted" ]; then \
		printf 'Go files need formatting:\n%s\n' "$$unformatted"; \
		exit 1; \
	fi
	git diff --check

##@ Linting

.PHONY: vet
vet: ## Run Go static analysis.
	go vet ./...

.PHONY: lint
lint: lint-go ## Run Go linters.

.PHONY: lint-go
lint-go: golangci-lint ## Run golangci-lint.
	$(call run-tool,$(GOLANGCI_LINT),run)

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci-lint and apply fixes.
	$(call run-tool,$(GOLANGCI_LINT),run --fix)

.PHONY: check
check: check-format vet test-race ## Run formatting, vet, and race checks.

.PHONY: verify
verify: check lint ## Run all local checks.

##@ Dependencies

.PHONY: dev-tools
dev-tools: $(DEV_PORT) $(DEV_TAG) $(OPEN_BROWSER) $(MAKE_HELP) $(GO_INSTALL_TOOL) ## Download the pinned shared development tools.

$(GOLANGCI_LINT): $(GO_INSTALL_TOOL)
	@mkdir -p "$(dir $@)"
	@$(GO_INSTALL_TOOL) \
		--target "$@" \
		--package github.com/golangci/golangci-lint/v2/cmd/golangci-lint \
		--tool-version "$(GOLANGCI_LINT_VERSION)"

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Install the pinned golangci-lint version.

$(KUMBUKA_CLI): $(GITHUB_RELEASE_INSTALL)
	@mkdir -p "$(dir $@)"
	@$(GITHUB_RELEASE_INSTALL) \
		--repo kumbuka-me/cli \
		--tag "$(KUMBUKA_CLI_VERSION)" \
		--asset "$(KUMBUKA_CLI_ASSET)" \
		--binary kumbuka-cli \
		--target "$@"

.PHONY: cli
cli: $(KUMBUKA_CLI) ## Download the pinned Kumbuka CLI release.
