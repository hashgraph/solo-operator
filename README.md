# solo-operator

[![GitHub Release](https://img.shields.io/github/v/release/hashgraph/solo-operator?logo=github\&label=Release)](https://github.com/hashgraph/solo-operator/releases/latest)
[![GitHub License](https://img.shields.io/github/license/hashgraph/solo-operator?logo=apache\&logoColor=red\&label=License)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.26+-blue?logo=go)](https://go.dev/)
[![Build Application](https://github.com/hashgraph/solo-operator/actions/workflows/flow-build-application.yaml/badge.svg)](https://github.com/hashgraph/solo-operator/actions/workflows/flow-build-application.yaml)


> The opinionated Kubernetes operator powering the Solo toolkit.

## Table of Contents

  - [Overview](#overview)
  - [Components](#components)
  - [Custom Resources](#custom-resources)
  - [Operator Installation](#operator-installation)
    - [Helm Chart](#helm-chart)
    - [Kubernetes Manifest](#kubernetes-manifest)
  - [Getting Started](#getting-started)
  - [Development](#development)
  - [Release Management](#release-management)
  - [Support](#support)
  - [Contributing](#contributing)
  - [Code of Conduct](#code-of-conduct)
  - [License](#license)

## Overview

Solo Operator automates the deployment and lifecycle management of Hedera consensus networks on Kubernetes. Built with Go using [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime), it provides Custom Resource Definitions (CRDs) for managing all components of a Hedera network.

## Components

This repository builds and releases several artifacts, all at the **same version in lockstep**:

| Artifact | Source | Published to | Description |
|----------|--------|--------------|-------------|
| **Operator** image | `cmd/main.go` | `ghcr.io/hashgraph/solo-operator/solo-operator` | The controller manager that reconciles the CRDs |
| **UC sidecar** image | `cmd/uc/` | `ghcr.io/hashgraph/solo-operator/uc` | Upgrade-controller sidecar for the network upgrade flow ([docs](docs/uc/)) |
| **beacon** image | `cmd/beacon/` | `ghcr.io/hashgraph/solo-operator/beacon` | HAPI control-plane CLI for privileged consensus-node upgrade transactions ([docs](docs/beacon/README.md)) |
| **Helm chart** | `charts/solo-operator/` | `oci://ghcr.io/hashgraph/solo-operator/solo-operator-chart` | Deploys the operator (see [Installation](#operator-installation)) |
| **CRDs** subchart | `charts/solo-operator/charts/crds/` | bundled in the Helm chart | Generated CRD manifests, installed with the chart |

Notes on how artifacts are published:

- **JFrog mirror.** The three images and the Helm chart are also published to JFrog
  Artifactory: images to `artifacts.hashgraph.io/solo-operator-docker-release/{solo-operator,uc,beacon}`
  and the chart to `oci://artifacts.hashgraph.io/solo-operator-helm-release/solo-operator-chart`.
- **Immutable, version-tagged.** Every image and chart is tagged only by its release
  version (e.g. `:0.6.0`) — there is **no `:latest`** tag. Always pin an explicit version.
- **CRD manifests as release assets.** The generated CRD schemas (`config/crd/bases/*.yaml`)
  are attached to each [GitHub release](https://github.com/hashgraph/solo-operator/releases)
  in addition to being bundled in the Helm chart.

## Custom Resources

| Resource | Scope | Description |
|----------|-------|-------------|
| **Orbit** | Cluster | Network configuration and settings |
| **ConsensusCapsule** | Namespace | Hedera consensus node deployments |
| **NetworkGenesis** | Namespace | Network genesis operation (roster + address book ConfigMaps) |
| **HAProxyCapsule** | Namespace | HAProxy load balancer for gRPC traffic |
| **EnvoyProxy** | Namespace | Envoy proxy for HTTP/gRPC routing |

The CRD type definitions live in [`api/v1alpha1/`](api/v1alpha1/), and the generated
CRD schemas (with field-level documentation) are in [`config/crd/bases/`](config/crd/bases/).

### ConsensusCapsule Lifecycle & Actuation

The `ConsensusCapsule` resource defines how a Hedera consensus node starts and operates. This models the actuation definitions set out in HIP-1495.

**`spec.offline` vs `spec.desiredRunState`:**
- **`spec.offline`: `true`** - The node is fully offline. The StatefulSet is scaled to `0`. There is no pod, no resources consumed, and no background processes running.
- **`spec.desiredRunState`: `Stopped`** - The pod is running, but the main Java consensus node process is stopped (idle). Background services, sidecars, and logging utilities remain active via the s6-overlay init system. This is an active administrative state.
- **`spec.desiredRunState`: `Running`** - The pod is up and the full Java consensus node process is actively managed (started and supervised) by s6.

**Start Policy & Lifecycle Phases:**
- **`spec.startPolicy: Auto`** (Default) - The operator ensures the capsule rolls out and the Java process starts automatically upon pod boot.
- **`spec.startPolicy: Manual`** - The consensus node process will not start automatically. On creation (or restart), the capsule enters the **`Stopped`** phase holding the Java process idle until a manual actuation changes `spec.desiredRunState` to `Running`.

To transition a starting node that is held back via a manual policy, patch the CR:
```shell
kubectl patch consensuscapsule <name> -p '{"spec":{"desiredRunState":"Running"}}' --type=merge
```


## Operator Installation

The Solo Operator provides several different methods for installation into a Kubernetes cluster.

### Helm Chart

The Helm chart is published to two OCI registries on each release:

| Registry | Chart URL |
|----------|-----------|
| GitHub Container Registry (GHCR, public) | `oci://ghcr.io/hashgraph/solo-operator/solo-operator-chart` |
| JFrog Artifactory | `oci://artifacts.hashgraph.io/solo-operator-helm-release/solo-operator-chart` |

```shell
# Install latest (add --version <x.y.z> to pin a release)
helm install solo-operator oci://ghcr.io/hashgraph/solo-operator/solo-operator-chart \
  --namespace solo-operator --create-namespace

# Uninstall
helm uninstall solo-operator --namespace solo-operator
```

### CRD manifests (release assets)

The operator itself is installed via the Helm chart above — no standalone operator
Deployment manifest is published. Each [GitHub release](https://github.com/hashgraph/solo-operator/releases)
does attach the generated **CRD schemas** (`config/crd/bases/*.yaml`) as individual assets,
which you can apply directly if you manage CRDs out-of-band from the chart (`crds.install=false`):

```shell
# Apply a specific CRD from a pinned release (repeat per CRD, or loop the release assets)
kubectl apply -f https://github.com/hashgraph/solo-operator/releases/download/vX.Y.Z/operator.solo.hiero.org_orbits.yaml
```

## Getting Started

To deploy a local 4-node test network against the operator, follow the
[example deployment guide](docs/example/README.md).

## Development

See the [development guide](docs/development.md) for prerequisites, project
structure, build/test tasks, and running the operator locally. Operator
configuration flags are documented in [docs/configuration.md](docs/configuration.md).

## Release Management

This project uses the [semantic-release](https://github.com/semantic-release/semantic-release) tool to automate the
versioning and release processes. Version increments are determined by the
[conventional commits](https://www.conventionalcommits.org/en/v1.0.0/) present in the commit history.

### Creating a Release

1. Ensure `main` branch contains conventional commit messages (e.g., `fix:`, `feat:`, `BREAKING CHANGE:`).
2. Execute the [Release: Deploy Artifact](https://github.com/hashgraph/solo-operator/actions/workflows/flow-release-deploy-artifact.yaml) workflow.
3. The workflow will:
   - Determine the next version using semantic-release
   - Create a release tag and GitHub release, attaching the CRD manifests (`config/crd/bases/*.yaml`) as assets
   - Build and publish the **operator**, **UC sidecar**, and **beacon** images — to `ghcr.io/hashgraph/solo-operator/{solo-operator,uc,beacon}` and mirrored to `artifacts.hashgraph.io/solo-operator-docker-release/{solo-operator,uc,beacon}`
   - Publish the Helm chart to `oci://ghcr.io/hashgraph/solo-operator/solo-operator-chart` and `oci://artifacts.hashgraph.io/solo-operator-helm-release/solo-operator-chart`
   - All images and the chart are tagged with the **same release version** (no `:latest`) — operator, UC sidecar, beacon, and chart are released in lockstep

## Support

If you have a question on how to use the product, please see our [support guide](https://github.com/hashgraph/.github/blob/main/SUPPORT.md).

## Contributing

Contributions are welcome. Please see the [contributing guide](https://github.com/hashgraph/.github/blob/main/CONTRIBUTING.md) to see how you can get involved.

## Code of Conduct

This project is governed by the [Contributor Covenant Code of Conduct](https://github.com/hashgraph/.github/blob/main/CODE_OF_CONDUCT.md). By participating, you are
expected to uphold this code of conduct.

## License

[Apache License 2.0](LICENSE)
