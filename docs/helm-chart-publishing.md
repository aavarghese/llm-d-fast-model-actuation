# Helm Chart Publishing and Usage

## Overview

The FMA project provides two Helm charts published to GitHub Container Registry (GHCR) as OCI artifacts:

1. **dual-pods-controller** (`charts/dpctlr/`)
2. **launcher-populator** (`charts/launcher-populator/`)

Charts are published to: `oci://ghcr.io/llm-d-incubation/llm-d-fast-model-actuation/charts`

**Installation**:

1.
```bash
helm install dpctlr oci://ghcr.io/llm-d-incubation/llm-d-fast-model-actuation/charts/dual-pods-controller --version 0.1.0
```

See `charts/dpctlr/values.yaml` for complete list of configurable parameters.

2.
```bash
helm install launcher-populator oci://ghcr.io/llm-d-incubation/llm-d-fast-model-actuation/charts/launcher-populator --version 0.1.0
```

See `charts/launcher-populator/values.yaml` for complete list of configurable parameters.

## Versioning and Git Hash Traceability

### Automatic Version Management

The publishing workflow automatically creates unique chart versions by appending the git commit hash to the base version. This ensures every commit gets a unique, traceable version.


The Helm chart references container images. Before publishing a chart:
1. Ensure the referenced container images exist and are published
2. Update the `Image` value in `charts/dpctlr/values.yaml` to reference a specific, published image tag
3. Avoid using `:latest` tag in published charts for reproducibility

 The publishing workflow automatically updates the `appVersion` field with the git commit hash for traceability.
#### How It Works

When the workflow runs, it automatically:
1. Gets the current git commit hash (short form, e.g., `d1a7c8f`)
2. Reads the base version from Chart.yaml (e.g., `0.1.0`)
3. Appends the git hash to create a unique version (e.g., `0.1.0-d1a7c8f`)
4. Updates both `version` and `appVersion` in Chart.yaml
5. Publishes the chart with the unique version

**Example transformation**:

Before workflow (in repository):
```yaml
apiVersion: v2
name: dual-pods-controller
version: 0.1.0              # Base version
appVersion: "0.1.0"
```

After workflow (published chart):
```yaml
apiVersion: v2
name: dual-pods-controller
version: 0.1.0-d1a7c8f      # Unique version with git hash
appVersion: "d1a7c8f"        # Git commit hash
```

**Benefits**:
- ✅ Every commit gets a unique version - prevents overwrites
- ✅ Automatic versioning - no manual bumps needed per commit
- ✅ Full traceability - version contains git hash
- ✅ Semantic versioning compliant - uses pre-release identifier

### When to Update Base Version

Update the base version in Chart.yaml when making significant changes:
- **MAJOR** (0.1.0 → 1.0.0): Incompatible API changes
- **MINOR** (0.1.0 → 0.2.0): Backwards-compatible functionality additions
- **PATCH** (0.1.0 → 0.1.1): Backwards-compatible bug fixes

### Container Image References

The Helm chart references container images. Before publishing:
1. Ensure the referenced container images exist and are published
2. Update the `Image` value in `values.yaml` to reference a specific tag
3. Avoid using `:latest` tag in published charts for reproducibility

## Publishing Process

### Automatic Publishing (Recommended for Stable Releases)

The chart is published automatically via the `.github/workflows/helm-release.yaml` workflow when:
- Changes are pushed to `main` branch in the `charts/**` directory
- This typically happens when a PR is merged
- The workflow automatically creates a unique version by appending the git hash
- Each commit gets a unique chart version (e.g., `0.1.0-d1a7c8f`)
- The chart should reference published container images with specific tags
- Avoid publishing charts that reference `:latest` or non-existent images

### Manual Publishing (For Dev Testing or Special Cases)

To manually trigger a chart release:
1. Go to the [Actions tab](https://github.com/llm-d-incubation/llm-d-fast-model-actuation/actions)
2. Select "Helm Chart Release" workflow
3. Click "Run workflow"
4. Select the `main` branch
5. Click "Run workflow"

## Using the Published Charts

```bash
# Install with default values
helm install dpctlr oci://ghcr.io/llm-d-incubation/llm-d-fast-model-actuation/charts/dual-pods-controller --version 0.1.0

# Install with custom values
helm install dpctlr oci://ghcr.io/llm-d-incubation/llm-d-fast-model-actuation/charts/dual-pods-controller \
  --version 0.1.0 \
  --set Image=ghcr.io/llm-d-incubation/llm-d-fast-model-actuation-controller:v0.1.0 \
  --set SleeperLimit=3

# Install with values file
helm install dpctlr oci://ghcr.io/llm-d-incubation/llm-d-fast-model-actuation/charts/dual-pods-controller \
  --version 0.1.0 \
  -f my-values.yaml
```

### Viewing Available Versions

To see available chart versions, visit the GitHub Container Registry:
- Dual Pods Controller: https://github.com/llm-d-incubation/llm-d-fast-model-actuation/pkgs/container/llm-d-fast-model-actuation%2Fcharts%2Fdual-pods-controller
- Launcher Populator: https://github.com/llm-d-incubation/llm-d-fast-model-actuation/pkgs/container/llm-d-fast-model-actuation%2Fcharts%2Flauncher-populator

### Using the charts in llm-d-benchmark

```bash
# Pull charts locally
helm pull oci://ghcr.io/llm-d-incubation/llm-d-fast-model-actuation/charts/dual-pods-controller --version 0.1.0
helm pull oci://ghcr.io/llm-d-incubation/llm-d-fast-model-actuation/charts/launcher-populator --version 0.1.0

# Or install directly
helm install dpctlr oci://ghcr.io/llm-d-incubation/llm-d-fast-model-actuation/charts/dual-pods-controller --version 0.1.0
```

## Scenarios Supported

The published chart supports the following usage scenarios:

1. **Developer Testing**: Developers can test their local changes before merging
2. **CI/CD for PRs**: Automated testing of proposed changes
3. **Main Branch Testing**: Continuous testing of the main branch
4. **Benchmark Framework**: Integration with llm-d-benchmark repository

## Troubleshooting

### Chart Not Appearing After Push

- Check the [Actions tab](https://github.com/llm-d-incubation/llm-d-fast-model-actuation/actions) for workflow status
- Ensure the `version` in `Chart.yaml` was incremented (workflow skips existing versions)
- Verify changes were in the `charts/**` path
- Check that the PR was merged to `main` (not just pushed to a branch)

### Chart Installation Fails

- Check chart values match your cluster requirements
- Review Kubernetes RBAC permissions
- Ensure referenced container images exist and are accessible
- Verify you're using Helm 3.8+ (required for OCI support)
- Check authentication if packages are private

### Workflow Fails on First Run

The first time the workflow runs, packages may need to be made public:
1. Go to GitHub repository → Packages
2. Find the published chart packages
3. Make them public in package settings
4. Grant repository write access to packages
5. Link packages to the repository

### Cannot Push Chart - Version Already Exists

If you see an error about the version already existing:
- Increment the `version` field in `Chart.yaml`
- The workflow will not overwrite existing chart versions
- This is by design to prevent accidental overwrites

## Support

For issues or questions:
- Open an issue in the [GitHub repository](https://github.com/llm-d-incubation/llm-d-fast-model-actuation/issues)
- Refer to the troubleshooting guide above
