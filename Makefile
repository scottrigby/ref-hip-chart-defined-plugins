# Makefile for Chart-Defined Plugins Reference Implementation
#
# This is a standalone reference implementation repo. The Makefile can
# automatically fetch and build Helm with chart-defined plugin support.
#
# Quick start:
#   make setup            - Fetch and build Helm fork
#   make test             - Run all tests
#   make clean-all        - Clean everything

SHELL := /bin/bash

# =============================================================================
# Configuration
# =============================================================================

# Helm fork with chart-defined plugins support
HELM_FORK_REPO := https://github.com/scottrigby/helm.git
HELM_FORK_BRANCH := chart-defined-plugins
HELM_BIN := ./helm

# Plugin list - add new plugins here
PLUGINS := varsubst-render gotemplate-render sourcefiles-modifier test-processor

# Helm environment paths (deferred evaluation - helm binary may not exist at parse time)
PLUGINS_DIR = $(shell $(HELM_BIN) env HELM_PLUGINS 2>/dev/null)
HELM_CACHE_HOME = $(shell $(HELM_BIN) env HELM_CACHE_HOME 2>/dev/null)
CONTENT_CACHE = $(HELM_CACHE_HOME)/content
WAZERO_CACHE = $(HELM_CACHE_HOME)/wazero-build

# Build settings
WASM_BUILD_FLAGS := GOOS=wasip1 GOARCH=wasm
GO_BUILD_CMD := go build -buildmode=c-shared

# OCI registry (local testing)
# Default: 127.0.0.1:5001 (host machine)
# Container: host.containers.internal:5001 (Podman/Docker container)
OCI_REGISTRY ?= 127.0.0.1:5001

# Colors
GREEN := \033[0;32m
YELLOW := \033[0;33m
NC := \033[0m

# =============================================================================
# Help
# =============================================================================

.PHONY: help
help:
	@echo "Chart-Defined Plugins Reference Implementation"
	@echo ""
	@echo "Setup (run first):"
	@echo "  make setup                   Fetch and build Helm with plugin support"
	@echo "  make setup-helm              Just fetch/build Helm (if plugins already built)"
	@echo "  make update-deps             Update all chart dependencies (rebuild Chart.lock)"
	@echo ""
	@echo "Quick start:"
	@echo "  make test                    Run all tests (requires OCI registry)"
	@echo "  make clean-all               Clean everything"
	@echo ""
	@echo "Build:"
	@echo "  make build-plugins           Build all Wasm plugins"
	@echo ""
	@echo "OCI Integration tests (requires local OCI registry at $(OCI_REGISTRY)):"
	@echo "  make test-oci                Push to registry, download, render"
	@echo "  make oci-push-all            Push all plugins to registry"
	@echo "  make oci-verify              Verify OCI registry contents"
	@echo ""
	@echo "Individual tests:"
	@echo "  make test-basic              Test basic rendering"
	@echo "  make test-gotemplate         Test gotemplate plugin"
	@echo "  make test-sequential         Test sequential plugin handoff"
	@echo ""
	@echo "Clean:"
	@echo "  make clean                   Clean built wasm files"
	@echo "  make clean-helm              Remove helm binary"
	@echo "  make clean-cache             Clean content cache and wazero cache"
	@echo "  make clean-all               Clean everything"

# =============================================================================
# Setup - Fetch and build Helm with chart-defined plugins support
# =============================================================================

.PHONY: setup
setup: setup-helm build-plugins
	@echo -e "$(GREEN)Setup complete! Run 'make test' to verify.$(NC)"

.PHONY: setup-helm
setup-helm:
	@echo -e "$(GREEN)Fetching and building Helm with chart-defined plugins...$(NC)"
	@if [ -f "$(HELM_BIN)" ]; then \
		echo "Helm binary already exists. Use 'make clean-helm' to rebuild."; \
	else \
		BUILD_DIR="$$(pwd)/.helm-build" && \
		rm -rf "$$BUILD_DIR" && \
		mkdir -p "$$BUILD_DIR" && \
		echo "Cloning $(HELM_FORK_REPO) branch $(HELM_FORK_BRANCH)..." && \
		git clone --depth 1 --branch $(HELM_FORK_BRANCH) $(HELM_FORK_REPO) "$$BUILD_DIR" && \
		echo "Building Helm..." && \
		$(MAKE) -C "$$BUILD_DIR" build && \
		mv "$$BUILD_DIR/bin/helm" $(HELM_BIN) && \
		rm -rf "$$BUILD_DIR" && \
		echo -e "$(GREEN)Helm binary ready at $(HELM_BIN)$(NC)"; \
	fi
	@$(HELM_BIN) version

# =============================================================================
# Build targets
# =============================================================================

# Build a single plugin: make build-plugin PLUGIN=varsubst-render
.PHONY: build-plugin
build-plugin:
	@echo -e "$(GREEN)Building $(PLUGIN) plugin...$(NC)"
	cd plugins/$(PLUGIN) && $(WASM_BUILD_FLAGS) $(GO_BUILD_CMD) -o plugin.wasm .
	@ls -lh plugins/$(PLUGIN)/plugin.wasm

.PHONY: build-plugins
build-plugins:
	@for plugin in $(PLUGINS); do \
		$(MAKE) build-plugin PLUGIN=$$plugin; \
	done

# =============================================================================
# Test targets
# =============================================================================

.PHONY: test
test: test-oci
	@echo -e "$(GREEN)All tests passed!$(NC)"

.PHONY: test-oci
test-oci: clean-cache build-plugins oci-push-all test-oci-download test-oci-render
	@echo -e "$(GREEN)OCI integration tests passed!$(NC)"

.PHONY: oci-push-all
oci-push-all:
	@for plugin in $(PLUGINS); do \
		echo -e "$(GREEN)Pushing $$plugin to OCI registry...$(NC)"; \
		VERSION=$$(grep 'version:' $(CURDIR)/plugins/$$plugin/plugin.yaml | head -1 | awk '{print $$2}'); \
		TMPDIR=$$(mktemp -d); \
		mkdir -p "$$TMPDIR/$$plugin"; \
		cp $(CURDIR)/plugins/$$plugin/plugin.yaml "$$TMPDIR/$$plugin/"; \
		cp $(CURDIR)/plugins/$$plugin/plugin.wasm "$$TMPDIR/$$plugin/"; \
		(cd "$$TMPDIR" && tar czf $$plugin-$$VERSION.tgz $$plugin && \
			oras push --plain-http \
				--disable-path-validation \
				--artifact-type "application/vnd.helm.plugin.v1+json" \
				$(OCI_REGISTRY)/plugins/$$plugin:$$VERSION \
				"$$plugin-$$VERSION.tgz:application/vnd.oci.image.layer.v1.tar+gzip"); \
		rm -rf "$$TMPDIR"; \
	done
	@echo -e "$(GREEN)All plugins pushed to OCI registry$(NC)"

.PHONY: update-deps
update-deps:
	@echo -e "$(GREEN)Updating all chart dependencies...$(NC)"
	@for chart in charts/*/; do \
		if [ -f "$$chart/Chart.yaml" ]; then \
			echo "Updating $$chart"; \
			$(HELM_BIN) dependency update "$$chart" --plain-http 2>/dev/null || true; \
		fi; \
	done

.PHONY: test-oci-download
test-oci-download:
	@echo -e "$(GREEN)Testing OCI plugin download to content cache...$(NC)"
	$(HELM_BIN) dependency update charts/varsubst-chart/ --plain-http
	$(HELM_BIN) dependency update charts/gotemplate-chart/ --plain-http
	@echo -e "$(GREEN)Verifying content cache...$(NC)"
	@find "$(CONTENT_CACHE)" -name "*.plugin" -type f 2>/dev/null | head -1 | grep -q . || \
		(echo "FAIL: No .plugin files in content cache ($(CONTENT_CACHE))" && exit 1)
	@echo -e "$(GREEN)OK: Plugins downloaded to content cache$(NC)"

.PHONY: test-oci-render
test-oci-render:
	@echo -e "$(GREEN)Testing rendering from content cache...$(NC)"
	@OUTPUT=$$($(HELM_BIN) template my-release charts/varsubst-chart/) && \
	echo "$$OUTPUT" && \
	echo "$$OUTPUT" | grep -q 'kind: Deployment' || (echo "FAIL: No Deployment in output" && exit 1) && \
	echo -e "$(GREEN)OK: Rendered from content cache$(NC)"

.PHONY: oci-verify
oci-verify:
	@echo -e "$(GREEN)Verifying OCI registry contents...$(NC)"
	curl -s http://$(OCI_REGISTRY)/v2/_catalog | jq .

# =============================================================================
# Individual test targets
# =============================================================================

.PHONY: test-basic
test-basic:
	@echo -e "$(GREEN)Test: Basic Render Plugin$(NC)"
	$(HELM_BIN) template my-release charts/varsubst-chart/

.PHONY: test-values
test-values:
	@echo -e "$(GREEN)Test: Custom Values$(NC)"
	$(HELM_BIN) template my-release charts/varsubst-chart/ \
		--set replicas=5 --set image.repository=httpd --set image.tag=2.4

.PHONY: test-namespace
test-namespace:
	@echo -e "$(GREEN)Test: Custom Namespace$(NC)"
	$(HELM_BIN) template custom-name charts/varsubst-chart/ --namespace custom-ns

.PHONY: test-debug
test-debug:
	@echo -e "$(GREEN)Test: Debug Output$(NC)"
	$(HELM_BIN) template my-release charts/varsubst-chart/ --debug 2>&1 | head -30

.PHONY: test-gotemplate
test-gotemplate:
	@echo -e "$(GREEN)Test: Gotemplate Render Plugin$(NC)"
	$(HELM_BIN) template my-release charts/gotemplate-chart/

.PHONY: test-sequential
test-sequential:
	@echo -e "$(GREEN)Test: Sequential Plugin Handoff$(NC)"
	@OUTPUT=$$($(HELM_BIN) template my-release charts/sequential-plugins-test/) && \
	echo "$$OUTPUT" && \
	echo "" && \
	echo -e "$(GREEN)Verifying sequential handoff...$(NC)" && \
	echo "$$OUTPUT" | grep -q 'name: sourcefiles-modifier-summary' || (echo "FAIL: Missing sourcefiles-modifier-summary" && exit 1) && \
	echo "$$OUTPUT" | grep -q 'name: test-processor-summary' || (echo "FAIL: Missing test-processor-summary" && exit 1) && \
	echo "$$OUTPUT" | grep 'test-processor-summary' -A20 | grep -q 'filesReceived: "2"' || (echo "FAIL: Expected filesReceived: 2" && exit 1) && \
	echo -e "$(GREEN)OK: Sequential handoff verified$(NC)"

.PHONY: test-no-plugin
test-no-plugin:
	@echo -e "$(GREEN)Test: Chart Without Plugins$(NC)"
	@TMPDIR=$$(mktemp -d) && \
	mkdir -p "$$TMPDIR/templates" && \
	echo -e 'apiVersion: v2\nname: test-chart\nversion: 1.0.0' > "$$TMPDIR/Chart.yaml" && \
	echo -e 'apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: {{ .Release.Name }}\ndata:\n  key: value' > "$$TMPDIR/templates/configmap.yaml" && \
	$(HELM_BIN) template test "$$TMPDIR" && \
	rm -rf "$$TMPDIR"

.PHONY: test-all
test-all: test-basic test-values test-namespace test-debug test-no-plugin test-gotemplate test-sequential
	@echo -e "$(GREEN)All individual tests passed!$(NC)"

# =============================================================================
# Clean targets
# =============================================================================

.PHONY: clean
clean:
	@echo -e "$(YELLOW)Cleaning built wasm files...$(NC)"
	@for plugin in $(PLUGINS); do \
		rm -f plugins/$$plugin/plugin.wasm; \
	done
	rm -f plugins/echo-render/plugin.wasm

.PHONY: clean-cache
clean-cache:
	@echo -e "$(YELLOW)Cleaning content cache and wazero cache...$(NC)"
	rm -f "$(CONTENT_CACHE)"/*.plugin 2>/dev/null || true
	rm -rf "$(WAZERO_CACHE)" 2>/dev/null || true

.PHONY: clean-plugins
clean-plugins:
	@echo -e "$(YELLOW)Cleaning plugin directories...$(NC)"
	@for plugin in $(PLUGINS); do \
		rm -rf "$(PLUGINS_DIR)/versions/$$plugin"; \
	done

.PHONY: clean-helm
clean-helm:
	@echo -e "$(YELLOW)Cleaning helm binary...$(NC)"
	rm -f $(HELM_BIN)

.PHONY: clean-all
clean-all: clean clean-helm clean-cache clean-plugins
	@echo -e "$(YELLOW)All caches and artifacts cleaned$(NC)"

# =============================================================================
# Utility targets
# =============================================================================

.PHONY: list-plugins
list-plugins:
	@echo -e "$(GREEN)Installed plugins:$(NC)"
	$(HELM_BIN) plugin list

.PHONY: show-cache
show-cache:
	@echo -e "$(GREEN)Content cache:$(NC)"
	ls -la "$(CONTENT_CACHE)"/*.plugin 2>/dev/null || echo "  (empty)"
	@echo -e "$(GREEN)Wazero cache:$(NC)"
	ls -la "$(WAZERO_CACHE)" 2>/dev/null || echo "  (empty)"

# =============================================================================
# Mock ArtifactHub server
# =============================================================================

.PHONY: mock-artifacthub
mock-artifacthub:
	@echo -e "$(GREEN)Starting mock ArtifactHub server...$(NC)"
	go run ./mock-artifacthub --registry ghcr.io/scottrigby/ref-hip-chart-defined-plugins

.PHONY: mock-artifacthub-local
mock-artifacthub-local:
	@echo -e "$(GREEN)Starting mock ArtifactHub server (local registry)...$(NC)"
	go run ./mock-artifacthub --registry $(OCI_REGISTRY)

# =============================================================================
# Container e2e test (tests ArtifactHub integration from within container)
# =============================================================================

CONTAINER_OCI_REGISTRY := host.containers.internal:5001
MOCK_ARTIFACTHUB_PORT := 8765

.PHONY: test-container-e2e
test-container-e2e: build-plugins
	@echo -e "$(GREEN)Running container e2e test with mock ArtifactHub...$(NC)"
	@# Build mock server
	go build -o mock-server ./mock-artifacthub
	@# Start mock server in background
	./mock-server --registry $(CONTAINER_OCI_REGISTRY) --port $(MOCK_ARTIFACTHUB_PORT) &
	@sleep 2
	@# Push plugins to OCI registry (using container-accessible address)
	@for plugin in $(PLUGINS); do \
		VERSION=$$(grep 'version:' plugins/$$plugin/plugin.yaml | head -1 | awk '{print $$2}'); \
		TMPDIR=$$(mktemp -d); \
		mkdir -p "$$TMPDIR/$$plugin"; \
		cp plugins/$$plugin/plugin.yaml "$$TMPDIR/$$plugin/"; \
		cp plugins/$$plugin/plugin.wasm "$$TMPDIR/$$plugin/"; \
		(cd "$$TMPDIR" && tar czf $$plugin-$$VERSION.tgz $$plugin && \
			oras push --plain-http \
				--disable-path-validation \
				--artifact-type "application/vnd.helm.plugin.v1+json" \
				$(CONTAINER_OCI_REGISTRY)/plugins/$$plugin:$$VERSION \
				"$$plugin-$$VERSION.tgz:application/vnd.oci.image.layer.v1.tar+gzip"); \
		rm -rf "$$TMPDIR"; \
	done
	@# Test dependency update with mock ArtifactHub
	@rm -f "$(CONTENT_CACHE)"/*.plugin 2>/dev/null || true
	$(HELM_BIN) dependency update charts/container-test-chart/ --plain-http \
		--artifacthub-endpoint http://localhost:$(MOCK_ARTIFACTHUB_PORT)
	@# Verify plugin was cached
	@test -n "$$(find "$(HELM_CACHE_HOME)/content" -name '*.plugin' 2>/dev/null | head -1)" || \
		(echo "FAIL: No plugins in content cache" && exit 1)
	@echo -e "$(GREEN)Container e2e test passed!$(NC)"
	@# Clean up mock server
	@pkill -f "mock-server --registry" || true
