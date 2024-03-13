# solo-operator

[![GitHub Release](https://img.shields.io/github/v/release/hashgraph/solo-operator?logo=github\&label=Release)](https://github.com/hashgraph/solo-operator/releases/latest)
[![GitHub License](https://img.shields.io/github/license/hashgraph/solo-operator?logo=apache\&logoColor=red\&label=License)](LICENSE)
[![Java Version](https://img.shields.io/badge/Java%20Version-21%20LTS-blue)](https://adoptium.net/temurin/releases/?package=jdk&version=21)
[![Java JVM](https://img.shields.io/badge/Java%20JVM-adoptium-blue)](https://adoptium.net/temurin/releases/?package=jdk&version=21)
[![Build Application](https://github.com/hashgraph/solo-operator/actions/workflows/flow-build-application.yaml/badge.svg)](https://github.com/hashgraph/solo-operator/actions/workflows/flow-build-application.yaml)


> The opinionated Kubernetes operator powering the Solo toolkit.

## Table of Contents

  - [Operator Installation](#operator-installation)
    - [Helm Chart](#helm-chart)
      - [Helm Install (Latest Version)](#helm-install-latest-version)
      - [Helm Install (Specific Version)](#helm-install-specific-version)
      - [Helm Uninstall](#helm-uninstall)
    - [Kubernetes Manifest](#kubernetes-manifest)
      - [KubeCtl Install (Latest Version)](#kubectl-install-latest-version)
      - [KubeCtl Install (Specific Version)](#kubectl-install-specific-version)
      - [KubeCtl Uninstall (Latest Version)](#kubectl-uninstall-latest-version)
      - [KubeCtl Uninstall (Specific Version)](#kubectl-uninstall-specific-version)
    - [JSON Manifest (via KubeCtl)](#json-manifest-via-kubectl)
      - [JSON Install (Latest Version)](#json-install-latest-version)
      - [JSON Install (Specific Version)](#json-install-specific-version)
      - [JSON Uninstall (Latest Version)](#json-uninstall-latest-version)
      - [JSON Uninstall (Specific Version)](#json-uninstall-specific-version)
  - [Application Development](#application-development)
    - [Code Formatting](#code-formatting)
    - [Running the Unit Tests](#running-the-unit-tests)
    - [Running the application in dev mode](#running-the-application-in-dev-mode)
    - [Packaging and running the application](#packaging-and-running-the-application)
    - [Creating a native executable](#creating-a-native-executable)
  - [Support](#support)
  - [Contributing](#contributing)
  - [Code of Conduct](#code-of-conduct)
  - [License](#license)

## Operator Installation

The Solo Operator provides several different methods for installation into a Kubernetes cluster. Please see the sections below for the method that best suits your needs.

### Helm Chart

The Solo Operator is available as a Helm chart from the Swirlds Labs Helm OCI repository.

#### Helm Install (Latest Version)
```shell
helm install solo-operator oci://artifacts.swirldslabs.io/solo-operator-helm-release/solo-operator --namespace solo-operator --create-namespace
```

#### Helm Install (Specific Version)
```shell
helm install solo-operator oci://artifacts.swirldslabs.io/solo-operator-helm-release/solo-operator --version 0.1.0 --namespace solo-operator --create-namespace
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
kubectl apply -f https://github.com/hashgraph/solo-operator/releases/download/v0.1.0/deployment.yml
```

#### KubeCtl Uninstall (Latest Version)
```shell
kubectl delete -f https://github.com/hashgraph/solo-operator/releases/latest/download/deployment.yml
```

#### KubeCtl Uninstall (Specific Version)
```shell
kubectl delete -f https://github.com/hashgraph/solo-operator/releases/download/v0.1.0/deployment.yml
```

### JSON Manifest (via KubeCtl)

#### JSON Install (Latest Version)
```shell
kubectl apply -f https://github.com/hashgraph/solo-operator/releases/latest/download/deployment.json
```

#### JSON Install (Specific Version)
```shell
kubectl apply -f https://github.com/hashgraph/solo-operator/releases/download/v0.1.0/deployment.json
```

#### JSON Uninstall (Latest Version)
```shell
kubectl delete -f https://github.com/hashgraph/solo-operator/releases/latest/download/deployment.json
```

#### JSON Uninstall (Specific Version)
```shell
kubectl delete -f https://github.com/hashgraph/solo-operator/releases/download/v0.1.0/deployment.json
```


## Application Development

This project uses Quarkus, the Supersonic Subatomic Java Framework.

If you want to learn more about Quarkus, please visit the [Quarkus website](https://quarkus.io/).

### Code formatting
You can apply the repository code formatting rules by running:
```shell script
./gradlew spotlessApply
```

### Running the unit tests
You can run the unit tests by running:
```shell script
./gradlew test
```

### Running the application in dev mode

You can run your application in dev mode that enables live coding using:
```shell script
./gradlew quarkusDev
```

> **_NOTE:_**  Quarkus now ships with a Dev UI, which is available in dev mode only at http://localhost:8080/q/dev/.

### Packaging and running the application

The application can be packaged using:
```shell script
./gradlew build
```
It produces the `quarkus-run.jar` file in the `build/quarkus-app/` directory.
Be aware that it’s not an _über-jar_ as the dependencies are copied into the `build/quarkus-app/lib/` directory.

The application is now runnable using `java -jar build/quarkus-app/quarkus-run.jar`.

If you want to build an _über-jar_, execute the following command:
```shell script
./gradlew build -Dquarkus.package.type=uber-jar
```

The application, packaged as an _über-jar_, is now runnable using `java -jar build/*-runner.jar`.

### Creating a native executable

You can create a native executable using:
```shell script
./gradlew build -Dquarkus.package.type=native
```

Or, if you don't have GraalVM installed, you can run the native executable build in a container using:
```shell script
./gradlew build -Dquarkus.package.type=native -Dquarkus.native.container-build=true
```

You can then execute your native executable with: `./build/solo-operator-0.1.0-runner`

If you want to learn more about building native executables, please consult https://quarkus.io/guides/gradle-tooling.


## Release Management

This project uses the [semantic-release](https://github.com/semantic-release/semantic-release) tool to automate the
versioning and release processes. Version increments are determined by the
[conventional commits](https://www.conventionalcommits.org/en/v1.0.0/) present in the commit history.
Manual tag and release branch creation is therefore restricted to maintain the integrity of the release process.

Releases shall be issued from the `main` branch and will contain all commits/changes since the prior release. The `main`
branch should always reflect the `current` release train and should be considered stable. No release branch shall be
issued until the `main` branch advances to the `next` release train. The release branch will be created from the
most recent version tag of the `current` release train prior to the `main` branch advancing to the `next` release train.

Release branches may only be used to issue hotfixes and patches to a `non-current` release train. No new features or
breaking changes shall be introduced in a release branch. When possible changes must be backported from the `main`
branch to the appropriate release branches. An exception to this rule is when a fix only applies to a specific release.

### Creating a release (Main Branch Only)

In order to create a release, the following steps must be taken:

1. Ensure that the `main` branch is up-to-date with the latest changes AND contains at least one commit with a
   conventional commit message which will result in a version increase (eg: `fix:`, `feat:`, `BREAKING CHANGE:`).
2. If this is the first release since a new release train was started, then the `main` branch must contain at least one
    commit with a conventional commit message which will result in a major or minor version increase (eg: `feat:` or
    `BREAKING CHANGE:`).
3. Execute the [Release: Deploy Artifact](https://github.com/hashgraph/solo-operator/actions/workflows/flow-release-deploy-artifact.yaml)
   workflow ensuring the `main` branch is selected as the target of the workflow.
4. The `Release: Deploy Artifact` workflow will:
   - Run the `semantic-release` tool to determine the next version and generate a release notes file.
   - Update the Gradle build version.
   - Create a new release version tag and commit.
   - Push the release version tag and commit to the repository.
   - Publish the release notes to the GitHub release page.
   - Publish the release artifacts to the GitHub release page.
   - Publish the release artifacts to the JFrog Artifactory repositories.

### Creating a release (Release Branch Only)

In order to create a release, the following steps must be taken:

1. Ensure that the `release/X.Y` branch is up-to-date with the latest changes AND contains at least one commit with a
   conventional commit message which will result in a patch version increase (eg: `fix:`, `BREAKING CHANGE:`).
2. If the `main` branch has not yet had a release since the `release/X.Y` branch was created, then a release must be
   created on the `main` branch before the `release/X.Y` branch can be released.
   Please follow the steps in the [Creating a release (Main Branch Only)](#creating-a-release-main-branch-only) section before proceeding.
3. Execute the [Release: Deploy Artifact](https://github.com/hashgraph/solo-operator/actions/workflows/flow-release-deploy-artifact.yaml)
   workflow ensuring the `release/X.Y` branch is selected as the target of the workflow.
4. The `Release: Deploy Artifact` workflow will:
    - Run the `semantic-release` tool to determine the next version and generate a release notes file.
    - Update the Gradle build version.
    - Create a new release version tag and commit.
    - Push the release version tag and commit to the repository.
    - Publish the release notes to the GitHub release page.
    - Publish the release artifacts to the GitHub release page.
    - Publish the release artifacts to the JFrog Artifactory repositories.

### Starting a new release train

In order to start a new release train, the following steps must be taken:

1. Ensure the `main` branch is ready to advance to the `next` release train.
2. Ensure the `main` branch contains or will contain at least one commit with a conventional commit message which will
   result in a major or minor version increase (eg: `feat:` or `BREAKING CHANGE:`) since the last release.
3. Execute the [Release: Create Next Train](https://github.com/hashgraph/solo-operator/actions/workflows/flow-release-create-next-train.yaml)
   workflow ensuring the `main` branch is selected as the target of the workflow.
4. The `Release: Create Next Train` workflow will:
   - Create a new release branch from the most recent version tag of the `current` release train.
   - Update the Gradle build version to the next minor version increment and add the `SNAPSHOT` prerelease specifier.
   - Push the release branch to the repository.

## Support

If you have a question on how to use the product, please see our [support guide](https://github.com/hashgraph/.github/blob/main/SUPPORT.md).

## Contributing

Contributions are welcome. Please see the [contributing guide](https://github.com/hashgraph/.github/blob/main/CONTRIBUTING.md) to see how you can get involved.

## Code of Conduct

This project is governed by the [Contributor Covenant Code of Conduct](https://github.com/hashgraph/.github/blob/main/CODE_OF_CONDUCT.md). By participating, you are
expected to uphold this code of conduct.

## License

[Apache License 2.0](LICENSE)
