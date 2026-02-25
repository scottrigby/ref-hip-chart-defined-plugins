# Reference Implementation: HIP Chart-Defined Plugins

Reference implementation for [HIP-9999: Chart-Defined Plugins](https://github.com/helm/community/pull/XXX).

## Quick Start

```bash
# Prerequisites: Go 1.21+, local OCI registry at 127.0.0.1:5001

make setup  # Fetch Helm fork and build plugins
make test   # Run tests
make help   # Show all targets
```

## Structure

- **plugins/**: Reference render/v1 plugins (Wasm)
- **charts/**: Example charts using these plugins
- **sdk-examples/**: Programmatic SDK usage examples

## Example Chart.yaml

```yaml
apiVersion: v3
name: my-chart
version: 1.0.0

plugins:
  - name: varsubst-render
    type: render/v1
    repository: oci://127.0.0.1:5001/helm-plugins/varsubst-render
    version: 0.1.0
```

## Related

- [HIP-9999: Chart-Defined Plugins](https://github.com/helm/community/pull/XXX)
- [Helm Fork](https://github.com/scottrigby/helm/tree/chart-defined-plugins)

## License

Apache License 2.0
