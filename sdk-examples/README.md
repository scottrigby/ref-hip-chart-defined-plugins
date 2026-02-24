# SDK Examples

These examples demonstrate how to use the Helm render plugin SDK programmatically.

## Prerequisites

These examples require the `chart-defined-plugins` branch of Helm v4:

- Fork: https://github.com/scottrigby/helm/tree/chart-defined-plugins

Each example has a `go.mod` with a `replace` directive pointing to the fork.
After the feature is merged upstream, you can remove the replace directive.

## Examples

### simple/

Basic SDK usage with default disk-based caches. Suitable for environments with
writable filesystems.

```go
renderer := &render.PluginRenderer{
    ContentCachePath: helmpath.CachePath("content"),
}
```

**Building:**

```bash
cd simple
go mod tidy
go build -o simple-render .
```

**Running:**

```bash
# Render a chart that uses render plugins
./simple-render /path/to/chart

# Example with varsubst-chart from this repo
./simple-render ../../charts/varsubst-chart
```

### non-writable-fs/

SDK usage for non-writable filesystem environments (serverless functions,
read-only containers, embedded systems).

**Required for non-writable filesystems:**

1. Set `ContentCachePath` to `""` (empty string) to disable disk access
2. Provide `PreloadedPlugins` with embedded plugin archives keyed by digest

```go
renderer := &render.PluginRenderer{
    ContentCachePath: "",  // Disable disk-based plugin cache

    // In-memory Wasm compilation cache
    CompilationCache: wazero.NewCompilationCache(),

    // Pre-loaded plugin archives (embedded at compile time)
    PreloadedPlugins: preloadedPlugins,
}
```

**Building:**

First, place the required `.plugin` archive files in the `plugins/` directory.
The files should be named by their SHA256 digest (matching Chart.lock).

You can get the digest from the chart's Chart.lock and copy from the content cache:

```bash
cd non-writable-fs

# Get digest from Chart.lock (example: varsubst-chart)
DIGEST=$(grep 'digest:' ../../charts/varsubst-chart/Chart.lock | tail -1 | awk '{print $2}')

# Copy from content cache (use helm env for OS-portable path)
HELM_CACHE=$(helm env HELM_CACHE_HOME)
cp "${HELM_CACHE}/content/${DIGEST:0:2}/${DIGEST}.plugin" "plugins/${DIGEST}.plugin"

# Build with embedded plugins
go mod tidy
go build -o serverless-render .
```

**Running:**

```bash
# The binary has plugins embedded - no disk access needed for plugins
./serverless-render
```

Note: The non-writable-fs example currently loads the chart from disk for
simplicity. In a real serverless environment, you would also embed the chart
data or receive it as input.

## Development vs Production

The `go.mod` files have replace directives for both local development and production use.

**Local development** (default):

```go
replace helm.sh/helm/v4 => ../../../helm
```

**Production** (use the fork branch):

```go
replace helm.sh/helm/v4 => github.com/scottrigby/helm chart-defined-plugins
```

To switch to production mode, edit `go.mod` and comment/uncomment the appropriate
replace directive, then run `go mod tidy`.

Once the feature is merged upstream to `helm/helm`, remove both replace directives:

```bash
go mod edit -dropreplace helm.sh/helm/v4
go get helm.sh/helm/v4@latest
go mod tidy
```
