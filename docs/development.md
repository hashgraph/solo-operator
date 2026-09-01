# Development

Guide for building, testing, and running the Solo Operator locally. For usage
(installing and operating the released operator), see the
[README](../README.md).

## Prerequisites

- Go 1.26+
- Kubernetes 1.32+ (CRD `selectableFields` is used for upgrade-controller dedup; GA in 1.32)
- [Task](https://taskfile.dev/) (optional, for build automation)
- [kubectl](https://kubernetes.io/docs/tasks/tools/)
- [Helm](https://helm.sh/) (for chart development)

## Project Structure

```
.
├── api/v1alpha1/           # CRD type definitions
├── cmd/                    # Entry points (operator main.go, uc, beacon)
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

## Common Tasks

Most workflows are driven by [Task](https://taskfile.dev/); run `task --list` for the full set.

| Task | Purpose |
|------|---------|
| `task build` | Build all packages |
| `task docker:build` | Build the operator Docker image |
| `task test:unit` | Unit tests with coverage |
| `task test:integration` | Integration tests (requires envtest) |
| `task test:e2e` | E2E tests (requires a kind cluster) |
| `task generate` | Regenerate CRD manifests + DeepCopy methods (run after editing `api/v1alpha1/`) |
| `task lint` | Run golangci-lint (auto-fix where possible) |

After changes to `api/v1alpha1/` or `config/rbac/`, also run `task helm:sync-crds` and `task helm:sync-rbac` to sync the generated files into the Helm chart.

## Running Locally

```shell
kubectl apply -f config/crd/bases/            # install CRDs
go run cmd/main.go                            # run against the current kubeconfig
go run cmd/main.go --leader-elect --metrics-bind-address=:8080
```

See [configuration.md](configuration.md) for the full list of operator flags.

## Deploy a test network

For an end-to-end walkthrough of deploying a local 4-node network against the
operator, see the [example deployment guide](example/README.md).
