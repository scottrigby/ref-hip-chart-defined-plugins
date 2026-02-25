# GitHub Actions

Workflows for releasing plugins and charts to GHCR OCI registry.

**Registry paths:**

- Plugins: `oci://ghcr.io/scottrigby/ref-hip-chart-defined-plugins/plugins/PLUGIN_NAME:VERSION`
- Charts: `oci://ghcr.io/scottrigby/ref-hip-chart-defined-plugins/charts/CHART_NAME:VERSION`

## Workflows

| Workflow      | Trigger      | What it does                                         |
| ------------- | ------------ | ---------------------------------------------------- |
| `ci.yml`      | PR / push    | Builds Wasm, validates plugins/charts, version check |
| `release.yml` | Push to main | Packages and pushes to GHCR, creates tags            |

## Initial Setup

### 1. Create PAT (Required)

PAT is required for workflows to create tags and for tag events to trigger workflows.

1. GitHub -> Settings -> Developer settings -> Personal access tokens -> Fine-grained tokens
2. Select this repository
3. Permissions: `contents: write`
4. Copy the token

### 2. Add GitHub Secrets

Repository -> Settings -> Secrets and variables -> Actions -> **Secrets**:

| Secret               | Value                                   |
| -------------------- | --------------------------------------- |
| `PAT`                | Personal Access Token (required)        |
| `GPG_KEYRING_BASE64` | Contents of `keyring.base64` (optional) |
| `GPG_PASSPHRASE`     | Your passphrase (optional)              |

### 3. Optional: GPG Signing Setup

Generate GPG key in a container sandbox:

```bash
podman run --rm -it -v $PWD:/output:Z alpine:latest sh
apk add --no-cache gnupg
gpg --full-generate-key
# Select: RSA/RSA, 4096, no expiration, your email, passphrase

KEYID=$(gpg --list-secret-keys --keyid-format=long | awk '/^sec/{print $2}' | cut -d'/' -f2)
gpg --armor --export $KEYID > /output/public.asc
gpg --export-secret-keys $KEYID | base64 -w 0 > /output/keyring.base64
exit
```

Add variables (Settings -> Variables tab):

| Variable            | Value             |
| ------------------- | ----------------- |
| `SIGN_PLUGINS`      | `true`            |
| `SIGNING_KEY_EMAIL` | Email used in key |

Clean up: `rm -f public.asc keyring.base64`

## Testing the Workflows

### First-Time Setup

1. **Push changes to main** to trigger the release workflow:

   ```bash
   git push origin main
   ```

2. **Watch the workflow** at:
   `https://github.com/scottrigby/ref-hip-chart-defined-plugins/actions`

3. **After first successful release, link packages to repository:**
   - Go to: `https://github.com/scottrigby?tab=packages`
   - Click on each package (e.g., `ref-hip-chart-defined-plugins/plugins/gotemplate-render`)
   - Package settings (gear icon) -> "Connect to a repository"
   - Select `scottrigby/ref-hip-chart-defined-plugins`

### Verify OCI Packages

After release, verify packages are accessible:

```bash
# List available tags
oras repo tags ghcr.io/scottrigby/ref-hip-chart-defined-plugins/plugins/gotemplate-render
oras repo tags ghcr.io/scottrigby/ref-hip-chart-defined-plugins/charts/gotemplate-chart

# Pull and inspect a plugin
oras pull ghcr.io/scottrigby/ref-hip-chart-defined-plugins/plugins/gotemplate-render:0.1.0

# Pull a chart using Helm
helm pull oci://ghcr.io/scottrigby/ref-hip-chart-defined-plugins/charts/gotemplate-chart --version 1.0.0
```

### Testing Changes

To test new plugin/chart releases:

1. Bump version in `plugin.yaml` or `Chart.yaml`
2. Push to main
3. Watch Actions for release workflow
4. Verify new tags appear: `https://github.com/scottrigby/ref-hip-chart-defined-plugins/tags`

## Tag Format

- Plugins: `plugin/PLUGIN_NAME-VERSION` (e.g., `plugin/gotemplate-render-0.1.0`)
- Charts: `chart/CHART_NAME-VERSION` (e.g., `chart/gotemplate-chart-1.0.0`)

## Troubleshooting

### "Tag already exists" - skipped release

If you see this message, the version wasn't bumped. Update the version in `plugin.yaml` or `Chart.yaml`.

### Package not visible / permission denied

After first release, packages default to private. To fix:

1. Go to package settings
2. Change visibility to "Public" (or add collaborators for private)

### Workflow not triggering on push

Ensure `paths` in workflow match your changes:

- `ci.yml` triggers on `plugins/**`, `charts/**`, `Makefile`
- `release.yml` triggers on `plugins/**`, `charts/**`
