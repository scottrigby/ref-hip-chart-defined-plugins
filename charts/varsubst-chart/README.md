# Pkl Example Chart

This is an example Helm chart that demonstrates using the Pkl render plugin
for chart-defined plugins in Helm 4.

## Overview

This chart uses:

- **Chart-defined plugins**: The `plugins` field in Chart.yaml specifies the
  varsubst-render plugin as a required dependency.
- **render/v1 plugin type**: The varsubst-render plugin processes `.pkl` files and
  returns rendered Kubernetes manifests.

## Prerequisites

- Helm 4 with chart-defined plugin support
- The varsubst-render plugin installed (automatically downloaded via `helm dependency update`)

## Installation

1. Update dependencies (this will download the varsubst-render plugin):

   ```bash
   helm dependency update ./pkl-example
   ```

2. Install the chart:

   ```bash
   helm install my-release ./pkl-example
   ```

## Templates

This chart includes Pkl templates instead of Go templates:

- `templates/deployment.pkl` - A Kubernetes Deployment
- `templates/service.pkl` - A Kubernetes Service

The varsubst-render plugin processes these files and outputs standard YAML manifests.

## Values

| Parameter          | Description                | Default     |
| ------------------ | -------------------------- | ----------- |
| `replicas`         | Number of replicas         | `3`         |
| `image.repository` | Container image repository | `nginx`     |
| `image.tag`        | Container image tag        | `1.24`      |
| `service.type`     | Kubernetes Service type    | `ClusterIP` |
| `service.port`     | Service port               | `80`        |

## How It Works

1. When you run `helm template` or `helm install`, Helm loads the chart
2. Helm checks the `plugins` field and finds the varsubst-render plugin requirement
3. The varsubst-render plugin is loaded from the versioned plugin cache
4. Template files matching the plugin's patterns (`*.pkl`) are passed to the plugin
5. The plugin renders the Pkl files and returns Kubernetes manifests
6. Helm includes the rendered manifests in the final output

## Development

To modify this example:

1. Edit the Pkl templates in `templates/`
2. Run `helm template ./pkl-example` to see the rendered output
3. Adjust values in `values.yaml` as needed
