# solo-operator

[![GitHub Release](https://img.shields.io/github/v/release/hashgraph/solo-operator?logo=github\&label=Release)](https://github.com/hashgraph/solo-operator/releases/latest)
[![GitHub License](https://img.shields.io/github/license/hashgraph/solo-operator?logo=apache\&logoColor=red\&label=License)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.26+-blue?logo=go)](https://go.dev/)
[![Build Application](https://github.com/hashgraph/solo-operator/actions/workflows/flow-build-application.yaml/badge.svg)](https://github.com/hashgraph/solo-operator/actions/workflows/flow-build-application.yaml)


> The opinionated Kubernetes operator powering the Solo toolkit.

## Table of Contents

  - [Overview](#overview)
  - [Custom Resources](#custom-resources)
  - [Operator Installation](#operator-installation)
    - [Helm Chart](#helm-chart)
    - [Kubernetes Manifest](#kubernetes-manifest)
  - [Application Development](#application-development)
    - [Prerequisites](#prerequisites)
    - [Project Structure](#project-structure)
    - [Building](#building)
    - [Testing](#testing)
    - [Code Generation](#code-generation)
    - [Running Locally](#running-locally)
  - [Configuration](#configuration)
  - [Release Management](#release-management)
  - [Support](#support)
  - [Contributing](#contributing)
  - [Code of Conduct](#code-of-conduct)
  - [License](#license)

## Overview

Solo Operator automates the deployment and lifecycle management of Hedera consensus networks on Kubernetes. Built with Go using [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime), it provides Custom Resource Definitions (CRDs) for managing all components of a Hedera network.

## Custom Resources

| Resource | Scope | Description |
|----------|-------|-------------|
| **Orbit** | Cluster | Network configuration and settings |
| **ConsensusCapsule** | Namespace | Hedera consensus node deployments |
| **NetworkGenesis** | Namespace | Network genesis operation (roster + address book ConfigMaps) |
| **HAProxyCapsule** | Namespace | HAProxy load balancer for gRPC traffic |
| **EnvoyProxy** | Namespace | Envoy proxy for HTTP/gRPC routing |

See the [API Reference](docs/api-reference.md) for detailed documentation.

## Operator Installation

The Solo Operator provides several different methods for installation into a Kubernetes cluster.

### Helm Chart

The Solo Operator Helm chart is published to two OCI registries on each release:

| Registry | URL |
|----------|-----|
| GitHub Container Registry (GHCR) | `oci://ghcr.io/hashgraph/charts` |
| JFrog Artifactory | `oci://artifacts.hashgraph.io/solo-operator-helm-release` |

#### Helm Install (Latest Version)
```shell
# From GHCR (public)
helm install solo-operator oci://ghcr.io/hashgraph/charts/solo-operator --namespace solo-operator --create-namespace

# From JFrog Artifactory
helm install solo-operator oci://artifacts.hashgraph.io/solo-operator-helm-release/solo-operator --namespace solo-operator --create-namespace
```

#### Helm Install (Specific Version)
```shell
# From GHCR (public)
helm install solo-operator oci://ghcr.io/hashgraph/charts/solo-operator --version 0.2.0 --namespace solo-operator --create-namespace

# From JFrog Artifactory
helm install solo-operator oci://artifacts.hashgraph.io/solo-operator-helm-release/solo-operator --version 0.2.0 --namespace solo-operator --create-namespace
```

#### Helm Uninstall
```shell
helm uninstall solo-operator --namespace solo-operator
```

### Kubernetes Manifest

#### KubeCtl Install (Latest Version)
```shell
kubectl apply -f https://github.com/hashgraph/solo-operator/releases/latest/download/deployment.yml
```

#### KubeCtl Install (Specific Version)
```shell
kubectl apply -f https://github.com/hashgraph/solo-operator/releases/download/v1.0.0/deployment.yml
```

#### KubeCtl Uninstall
```shell
kubectl delete -f https://github.com/hashgraph/solo-operator/releases/latest/download/deployment.yml
```

## Application Development

### Prerequisites

- Go 1.26+
- Kubernetes 1.32+ (CRD `selectableFields` is used for upgrade-controller dedup; GA in 1.32)
- [Task](https://taskfile.dev/) (optional, for build automation)
- [kubectl](https://kubernetes.io/docs/tasks/tools/)
- [Helm](https://helm.sh/) (for chart development)

### Project Structure

```
.
├── api/v1alpha1/           # CRD type definitions
├── cmd/                    # Main entry point
├── config/
│   ├── crd/bases/          # Generated CRD manifests
│   ├── rbac/               # RBAC manifests
│   └── manager/            # Manager deployment
├── internal/
│   ├── controller/         # Reconciler implementations
│   ├── subreconciler/      # Dependent resource pattern
│   ├── resources/          # Resource builders
│   └── operations/         # NetworkGenesis builders (roster, address book, cluster-config)
├── charts/solo-operator/   # Helm chart
├── docs/                   # Documentation
└── test/                   # Integration and E2E tests
```

### Building

```shell
# Build with Task
task build

# Or with Go directly
go build -o bin/manager cmd/main.go

# Build Docker image
task docker:build
# Or: docker build -t solo-operator:latest .
```

### Testing

```shell
# Run unit tests
task test
# Or: go test ./...

# Run with coverage
go test ./... -coverprofile=cover.out

# Run integration tests (requires envtest)
task test:integration

# Run E2E tests (requires kind cluster)
task test:e2e

# Run benchmarks
go test -bench=. ./test/benchmark/...
```

### Code Generation

```shell
# Generate all (CRDs + DeepCopy)
task generate

# Generate CRD manifests only
task generate:manifests

# Generate DeepCopy methods only
task generate:deepcopy
```

### Running Locally

```shell
# Install CRDs
kubectl apply -f config/crd/bases/

# Run the operator (uses current kubeconfig)
go run cmd/main.go

# With specific flags
go run cmd/main.go --leader-elect --metrics-bind-address=:8080
```

### Linting

```shell
task lint
# Or: golangci-lint run
```

## Configuration

The operator accepts the following flags:

| Flag | Description | Default |
|------|-------------|---------|
| `--leader-elect` | Enable leader election for HA | `false` |
| `--metrics-bind-address` | Metrics endpoint address | `:8080` |
| `--health-probe-bind-address` | Health probe endpoint | `:8081` |
| `--zap-log-level` | Log level (debug, info, error) | `info` |
| `--zap-encoder` | Log encoder (json, console) | `console` |

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
