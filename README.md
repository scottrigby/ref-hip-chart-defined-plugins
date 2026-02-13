# Reference Implementation: HIP Chart-Defined Plugins

Reference implementation for [HIP-9999: Chart-Defined Plugins](https://github.com/helm/community/pull/XXX),
demonstrating Helm 4's chart-defined plugin system with render/v1 plugins.

This repository contains example Wasm plugins and charts that showcase the plugin
system's capabilities.

## Quick Start

### Prerequisites

- Go 1.21+ (with wasip1 support)
- `oras` CLI (for OCI testing) - [install](https://oras.land/docs/installation)
- Local OCI registry (for OCI testing) - `docker run -d -p 5001:5000 registry:2`

### Setup and Test

```bash
# Fetch Helm fork and build plugins
make setup

# Run tests (requires local OCI registry at 127.0.0.1:5001)
make test

# Or test without OCI registry (uses local copy)
make test-fallback-path
```

### Manual Setup

If you prefer to build Helm manually:

```bash
# Clone the Helm fork with render/v1 support
git clone -b chart-defined-plugins https://github.com/scottrigby/helm.git /tmp/helm-fork
cd /tmp/helm-fork && make build
cp bin/helm /path/to/this/repo/helm

# Build plugins
make build-plugins
```

## Overview

This repository demonstrates:

- **plugins/**: Reference render/v1 plugins (Wasm)
- **charts/**: Example Helm charts using these plugins

## Plugins

| Plugin                        | Description                                      |
| ----------------------------- | ------------------------------------------------ |
| `gotemplate-render`           | Gotemplate rendering as a Wasm plugin            |
| `varsubst-render`             | Variable substitution (placeholder for real Pkl) |
| `sourcefiles-modifier`        | Test: demonstrates SourceFiles modification      |
| `test-processor`              | Test: verifies received files                    |
| `globally-installed-fallback` | Test: fallback to globally installed plugins     |

## Using Plugins in Charts

### Chart.yaml

```yaml
apiVersion: v2
name: my-chart
version: 1.0.0

plugins:
  - name: varsubst-render
    type: render/v1
    repository: oci://ghcr.io/helm/plugins/varsubst-render
    version: 0.1.0
```

### Install Plugin Dependencies

```bash
helm dependency update ./my-chart
```

### Render Templates

```bash
helm template my-release ./my-chart
```

## Plugin Interface (render/v1)

### Input (InputMessageRenderV1)

```json
{
  "release": { "name": "my-release", "namespace": "default" },
  "values": { "replicas": 3 },
  "chart": { "name": "my-chart", "version": "1.0.0" },
  "capabilities": { "kubeVersion": { "version": "v1.28.0" } },
  "sourceFiles": [{ "name": "templates/deployment.pkl", "data": "..." }]
}
```

### Output (OutputMessageRenderV1)

```json
{
  "renderedFiles": {
    "templates/deployment.yaml": "apiVersion: apps/v1\n..."
  }
}
```

### Plugin Configuration (plugin.yaml)

```yaml
apiVersion: v1
name: my-plugin
version: 0.1.0
runtime: extism/v1
type: render/v1
config:
  patterns:
    - "templates/*.pkl"
```

## Project Structure

```
.
├── Makefile              # Build and test automation
├── helm                  # Helm binary (auto-fetched, gitignored)
├── plugins/              # Reference render plugins
│   ├── gotemplate-render/
│   ├── varsubst-render/
│   ├── sourcefiles-modifier/
│   ├── test-processor/
│   └── globally-installed-fallback/
├── charts/               # Example charts
│   ├── varsubst-chart/
│   ├── gotemplate-chart/
│   ├── sequential-plugins-test/
│   └── fallback-test-chart/
└── README.md
```

## Makefile Targets

| Target                    | Description                              |
| ------------------------- | ---------------------------------------- |
| `make setup`              | Fetch Helm fork and build all plugins    |
| `make test`               | Run OCI integration tests                |
| `make test-fallback-path` | Run tests without OCI registry           |
| `make build-plugins`      | Build all Wasm plugins                   |
| `make oci-push-all`       | Push plugins to local OCI registry       |
| `make clean-all`          | Clean everything (helm, plugins, caches) |

## Plugin Storage Architecture

Chart-defined plugins use content-addressable storage:

| Storage Type      | Path                                       | Purpose                                 |
| ----------------- | ------------------------------------------ | --------------------------------------- |
| **Content Cache** | `$HELM_CACHE_HOME/content/{digest}.plugin` | Plugin tarballs stored by SHA256 digest |
| **Wazero Cache**  | `$HELM_CACHE_HOME/wazero-build/`           | JIT-compiled Wasm modules (8x faster)   |
| **Fallback**      | `$HELM_PLUGINS/<name>/`                    | Globally installed plugins              |

## Related

- [HIP-9999: Chart-Defined Plugins](https://github.com/helm/community/pull/XXX)
- [HIP-0026: Wasm Plugin System](https://github.com/helm/community/blob/main/hips/hip-0026.md)
- [Helm Fork (chart-defined-plugins branch)](https://github.com/scottrigby/helm/tree/chart-defined-plugins)

## License

Apache License 2.0
