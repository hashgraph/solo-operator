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
| **Operator** image | `cmd/main.go` | `ghcr.io/hashgraph/solo-operator` | The controller manager that reconciles the CRDs |
| **UC sidecar** image | `cmd/uc/` | `ghcr.io/hashgraph/solo-operator-uc` | Upgrade-controller sidecar for the network upgrade flow ([docs](docs/uc/)) |
| **beacon** image | `cmd/beacon/` | `ghcr.io/hashgraph/solo-operator-beacon` | HAPI control-plane CLI for privileged consensus-node upgrade transactions ([docs](docs/beacon/README.md)) |
| **Helm chart** | `charts/solo-operator/` | `oci://ghcr.io/hashgraph/charts/solo-operator` | Deploys the operator (see [Installation](#operator-installation)) |
| **CRDs** subchart | `charts/solo-operator/charts/crds/` | bundled in the Helm chart | Generated CRD manifests, installed with the chart |

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

## Operator Installation

The Solo Operator provides several different methods for installation into a Kubernetes cluster.

### Helm Chart

The Helm chart is published to two OCI registries on each release:

| Registry | Chart URL |
|----------|-----------|
| GitHub Container Registry (GHCR, public) | `oci://ghcr.io/hashgraph/charts/solo-operator` |
| JFrog Artifactory | `oci://artifacts.hashgraph.io/solo-operator-helm-release/solo-operator` |

```shell
# Install latest (add --version <x.y.z> to pin a release)
helm install solo-operator oci://ghcr.io/hashgraph/charts/solo-operator \
  --namespace solo-operator --create-namespace

# Uninstall
helm uninstall solo-operator --namespace solo-operator
```

### Kubernetes Manifest

```shell
# Install latest (swap `latest/download` for `download/vX.Y.Z` to pin a release)
kubectl apply -f https://github.com/hashgraph/solo-operator/releases/latest/download/deployment.yml

# Uninstall
kubectl delete -f https://github.com/hashgraph/solo-operator/releases/latest/download/deployment.yml
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
   - Create a release tag and GitHub release
   - Build and publish container images to `ghcr.io/hashgraph/solo-operator` and `artifacts.hashgraph.io/solo-operator-docker-release/solo-operator`
   - Build and publish the UC sidecar image to `ghcr.io/hashgraph/solo-operator-uc` (and the JFrog mirror) at the **same** version — the operator, Helm chart, and UC sidecar are released in lockstep
   - Publish Helm chart to `oci://ghcr.io/hashgraph/charts/solo-operator` and `oci://artifacts.hashgraph.io/solo-operator-helm-release/solo-operator`

## Support

If you have a question on how to use the product, please see our [support guide](https://github.com/hashgraph/.github/blob/main/SUPPORT.md).

## Contributing

Contributions are welcome. Please see the [contributing guide](https://github.com/hashgraph/.github/blob/main/CONTRIBUTING.md) to see how you can get involved.

## Code of Conduct

This project is governed by the [Contributor Covenant Code of Conduct](https://github.com/hashgraph/.github/blob/main/CODE_OF_CONDUCT.md). By participating, you are
expected to uphold this code of conduct.

## License

[Apache License 2.0](LICENSE)
