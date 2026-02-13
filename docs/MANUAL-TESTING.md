# Manual Testing Guide

This document describes how to manually test the chart-defined plugin system.

## Prerequisites

1. Set up the repository (fetches Helm fork and builds plugins):

   ```bash
   make setup
   ```

   Or manually:

   ```bash
   # Clone and build Helm fork
   git clone -b chart-defined-plugins https://github.com/scottrigby/helm.git /tmp/helm-fork
   cd /tmp/helm-fork && make build
   cp bin/helm ./helm

   # Build plugins
   make build-plugins
   ```

2. For OCI-based testing (recommended), ensure you have:
   - A local OCI registry (e.g., `127.0.0.1:5001`)
   - `oras` CLI installed for pushing artifacts

   ```bash
   # Start a local registry
   docker run -d -p 5001:5000 --name registry registry:2
   ```

## Plugin Storage Architecture

Chart-defined plugins use content-addressable storage:

| Storage Type      | Path                                       | Purpose                                      |
| ----------------- | ------------------------------------------ | -------------------------------------------- |
| **Content Cache** | `$HELM_CACHE_HOME/content/{digest}.plugin` | Plugin tarballs stored by SHA256 digest      |
| **Wazero Cache**  | `$HELM_CACHE_HOME/wazero-build/`           | JIT-compiled Wasm modules (8x faster)        |
| **Fallback**      | `$HELM_PLUGINS/<name>/`                    | Globally installed plugins (directory-based) |

**Key point**: Chart-defined plugins are loaded **directly from tarballs** at runtime.
No extraction to disk is needed - the `plugin.yaml` and `.wasm` files are read into memory.

## Testing Approaches

### Approach A: OCI Registry (Primary - Tests Full Flow)

This tests the complete chart-defined plugin workflow:

1. Plugin pushed to OCI registry
2. `helm dependency update` downloads plugin to content cache
3. `helm template` loads plugin directly from cached tarball

```bash
# 1. Ensure OCI registry is running
curl http://127.0.0.1:5001/v2/_catalog

# 2. Build and push all plugins to registry
make build-plugins
make oci-push-all

# 3. Verify registry contents
make oci-verify
# Should show: {"repositories":["plugins/varsubst-render",...]}

# 4. Update chart dependencies (downloads plugin to content cache)
./helm dependency update charts/varsubst-chart/ --plain-http

# 5. Verify plugin is in content cache
find $(./helm env HELM_CACHE_HOME)/content -name "*.plugin"

# 6. Test rendering
./helm template my-release charts/varsubst-chart/
```

### Approach B: Local Copy (Tests Fallback Path)

This tests the versioned directory and global fallback loading (without OCI):

```bash
# Copy plugins to versioned directories
make copy-local-all

# Test rendering (uses directory-based loading)
./helm template my-release charts/varsubst-chart/
```

## Quick Test Commands

```bash
# Run full OCI integration tests
make test

# Run fallback path tests (no OCI registry needed)
make test-fallback-path
```

## Individual Test Cases

### Test 1: Basic Render Plugin

**Purpose**: Verify render plugin is invoked correctly.

```bash
make test-basic
# or
./helm template my-release charts/varsubst-chart/
```

**Expected**: Deployment and Service rendered with variable substitution.

### Test 2: Custom Values

**Purpose**: Verify user values are passed to plugin.

```bash
make test-values
# or
./helm template my-release charts/varsubst-chart/ \
  --set replicas=5 --set image.repository=httpd
```

**Expected**: `replicas: 5` and `image: "httpd:..."` in output.

### Test 3: Sequential Plugin Handoff

**Purpose**: Verify multiple plugins process files in order.

```bash
make test-sequential
```

**Expected**:

- Plugin 1 modifies SourceFiles (removes, renames, adds files)
- Plugin 2 receives only the modified file set
- Output shows which files each plugin processed

### Test 4: Fallback to Global Install

**Purpose**: Verify fallback when versioned path not found.

```bash
make test-fallback
```

**Expected**: Chart renders using globally installed plugin (non-versioned path).

### Test 5: Gotemplate as Render Plugin

**Purpose**: Verify gotemplate can be used as a render/v1 plugin.

```bash
make test-gotemplate
```

**Expected**: Standard gotemplate rendering via plugin system.

## Verifying Content Cache Loading

To confirm plugins are downloaded to the content cache:

```bash
# 1. Check content cache has plugin tarballs
CACHE_HOME=$(./helm env HELM_CACHE_HOME)
find "$CACHE_HOME/content" -name "*.plugin" -type f

# 2. Check versioned plugin directories (extracted from cache)
PLUGINS_DIR=$(./helm env HELM_PLUGINS)
ls -la "$PLUGINS_DIR/versions/"

# 3. Run with debug logging to see plugin loading
./helm template my-release charts/varsubst-chart/ --debug 2>&1 | grep -i plugin
```

## Cleanup

```bash
# Remove built wasm files
make clean

# Remove content cache and wazero cache
make clean-cache

# Remove versioned plugin directories
make clean-plugins

# Remove helm binary (to rebuild from fork)
make clean-helm

# Clean everything
make clean-all

# Stop local registry
docker stop registry && docker rm registry
```

## Troubleshooting

### Plugin Not Found

1. Check Chart.yaml has correct plugin repository URL
2. Run `helm dependency update --plain-http` to download plugin
3. Verify content cache: `find $(./helm env HELM_CACHE_HOME)/content -name "*.plugin"`
4. Verify versioned directory: `ls $(./helm env HELM_PLUGINS)/versions/`

### OCI Registry Issues

1. Verify registry is accessible: `curl http://127.0.0.1:5001/v2/_catalog`
2. Ensure `--plain-http` flag is used for HTTP registries
3. Check artifact type is `application/vnd.helm.plugin.v1+json`

### Wasm Execution Errors

1. Verify wasm file is valid: `file plugins/varsubst-render/plugin.wasm`
2. Check wazero cache: `ls $(./helm env HELM_CACHE_HOME)/wazero-build/`
3. Try clearing wazero cache and re-running

## Verification Checklist

- [ ] OCI download -> content cache -> archive loading works
- [ ] Fallback to globally installed plugins works
- [ ] Sequential plugin execution preserves file modifications
- [ ] Values, release info, and capabilities passed correctly
- [ ] Charts without plugins use default gotemplate engine
- [ ] Wazero compilation cache improves subsequent runs
