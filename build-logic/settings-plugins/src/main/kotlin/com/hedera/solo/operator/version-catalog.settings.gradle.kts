/*
 * Copyright (C) 2024 Hedera Hashgraph, LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package com.hedera.solo.operator

dependencyResolutionManagement {
    versionCatalogs {
        create("libs") {
            val assertjVersion = version("assertj", "3.25.3")
            val fabric8K8sClientVersion = version("fabric8-k8s-client", "6.13.0")
            val javaOperatorSdkVersion = version("java-operator-sdk", "4.9.1")
            val junitJupiterVersion = version("junit-jupiter", "5.10.0")
            val lombokVersion = version("lombok", "1.18.22")
            val quarkusPlatformVersion = version("quarkus-platform", "3.12.0")
            val sundrVersion = version("sundr", "0.103.1")
            library("assertj", "org.assertj", "assertj-core").versionRef(assertjVersion)
            library("fabric8-k8s-client", "io.fabric8", "kubernetes-client")
                .versionRef(fabric8K8sClientVersion)
            library("fabric8-k8s-client-bom", "io.fabric8", "kubernetes-client-bom")
                .versionRef(fabric8K8sClientVersion)
            library("fabric8-k8s-client-java-generator", "io.fabric8", "java-generator-core")
                .versionRef(fabric8K8sClientVersion)
            library("java-operator-sdk", "io.javaoperatorsdk", "operator-framework-bom")
                .versionRef(javaOperatorSdkVersion)
            library("junit-jupiter", "org.junit.jupiter", "junit-jupiter")
                .versionRef(junitJupiterVersion)
            library("lombok", "org.projectlombok", "lombok").versionRef(lombokVersion)
            library("quarkus-operator-sdk", "io.quarkus.platform", "quarkus-operator-sdk-bom")
                .versionRef(quarkusPlatformVersion)
            library("quarkus-platform", "io.quarkus.platform", "quarkus-bom")
                .versionRef(quarkusPlatformVersion)
            library("sundr-builder-annotations", "io.sundr", "builder-annotations")
                .versionRef(sundrVersion)
            plugin("quarkus", "io.quarkus").versionRef(quarkusPlatformVersion)
        }
    }
}
