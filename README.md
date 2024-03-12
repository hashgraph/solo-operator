# solo-operator

[![GitHub Release](https://img.shields.io/github/v/release/hashgraph/solo-operator)](https://github.com/hashgraph/solo-operator/releases/latest)
[![GitHub License](https://img.shields.io/github/license/hashgraph/solo-operator?logo=apache\&logoColor=red)](LICENSE)
[![Build Application](https://github.com/hashgraph/solo-operator/actions/workflows/flow-build-application.yaml/badge.svg)](https://github.com/hashgraph/solo-operator/actions/workflows/flow-build-application.yaml)

> The opinionated Kubernetes operator powering the Solo toolkit.

## Table of Contents

  - [Install Solo Operator](#install-solo-operator)
  - [Application Development](#application-development)
    - [Running the application in dev mode](#running-the-application-in-dev-mode)
    - [Packaging and running the application](#packaging-and-running-the-application)
    - [Creating a native executable](#creating-a-native-executable)
  - [Support](#support)
  - [Contributing](#contributing)
  - [Code of Conduct](#code-of-conduct)
  - [License](#license)

## Install Solo Operator

## Application Development

This project uses Quarkus, the Supersonic Subatomic Java Framework.

If you want to learn more about Quarkus, please visit the [Quarkus website](https://quarkus.io/).

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

## Support

If you have a question on how to use the product, please see our [support guide](https://github.com/hashgraph/.github/blob/main/SUPPORT.md).

## Contributing

Contributions are welcome. Please see the [contributing guide](https://github.com/hashgraph/.github/blob/main/CONTRIBUTING.md) to see how you can get involved.

## Code of Conduct

This project is governed by the [Contributor Covenant Code of Conduct](https://github.com/hashgraph/.github/blob/main/CODE_OF_CONDUCT.md). By participating, you are
expected to uphold this code of conduct.

## License

[Apache License 2.0](LICENSE)
