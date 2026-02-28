# Reference Implementation: HIP Chart-Defined Plugins

Reference implementation for [HIP-9999: Chart-Defined Plugins](https://github.com/helm/community/pull/XXX).

## Quick Start

```bash
# Prerequisites: Go 1.21+

make setup  # Fetch Helm fork and build plugins
make test   # Run tests (requires local OCI registry at 127.0.0.1:5001)
make help   # Show all targets
```

## Structure

- **plugins/**: Reference render/v1 plugins (Wasm)
- **charts/**: Example charts using these plugins
- **mock-artifacthub/**: Mock ArtifactHub server for testing plugin discovery

## Published Artifacts

Plugins and charts are published to GHCR:

```
oci://ghcr.io/scottrigby/ref-hip-chart-defined-plugins/plugins/<plugin-name>:<version>
oci://ghcr.io/scottrigby/ref-hip-chart-defined-plugins/charts/<chart-name>:<version>
```

## Example Chart.yaml

```yaml
apiVersion: v3
name: my-chart
version: 1.0.0

plugins:
  - name: varsubst-render
    type: render/v1
    repository: oci://ghcr.io/scottrigby/ref-hip-chart-defined-plugins/plugins/varsubst-render
    version: 0.1.3
```

## Mock ArtifactHub Server

The mock server simulates ArtifactHub's plugin discovery API for testing the trust workflow.

### Running the Mock Server

```bash
# Start mock server (discovers plugins from local ../plugins directory)
make mock-artifacthub

# Or with local OCI registry
make mock-artifacthub-local
```

### GHCR Authentication (Optional)

For direct OCI discovery from GHCR (instead of local fallback), set a GitHub token:

1. Create a [classic personal access token](https://github.com/settings/tokens)
2. Select scope: **read:packages**

```bash
export GITHUB_TOKEN=github_pat_xxxxx
make mock-artifacthub
```

Without the token, the mock server falls back to discovering plugins from the local `plugins/` directory.

### Testing the Trust Workflow

With the mock server running in one terminal, open another terminal:

```bash
# Clean any cached plugins
rm -rf $(./helm env HELM_CACHE_HOME)/content/*/*.plugin

# Run dependency update with mock ArtifactHub endpoint
./helm dependency update charts/varsubst-chart/ \
  --artifacthub-endpoint http://localhost:8080

# Verify rendering works
./helm template my-release charts/varsubst-chart/
```

Test the mock server API directly:

```bash
# Get plugin info
curl -s http://localhost:8080/api/v1/packages/helm-plugin/ref-hip-chart-defined-plugins/varsubst-render | jq .

# Health check
curl -s http://localhost:8080/health
```

## Related

- [HIP-9999: Chart-Defined Plugins](https://github.com/helm/community/pull/XXX)
- [Helm Fork](https://github.com/scottrigby/helm/tree/chart-defined-plugins)

## License

Apache License 2.0
