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

plugins {
    id("com.hedera.solo.operator.conventions")
    id("com.hedera.solo.operator.versioning")
    id("io.quarkus")
    id("com.hedera.solo.operator.buildlogic.objectModelGenerator")
}

val quarkusPlatformGroupId: String by project
val quarkusPlatformArtifactId: String by project
val quarkusPlatformVersion: String by project
val assertjVersion: String by project
val sundrVersion: String by project
val fabric8Version: String by project
val lombokVersion: String by project
val hederaSdkVersion: String by project
val javaoperatorsdkVersion: String by project

dependencies {
    annotationProcessor("io.sundr:builder-annotations:${sundrVersion}")
    annotationProcessor("io.fabric8:kubernetes-client:${fabric8Version}")
    annotationProcessor("io.fabric8:crd-generator-apt:${fabric8Version}")
    implementation("io.fabric8:generator-annotations:${fabric8Version}")

    implementation(
        enforcedPlatform(
            "${quarkusPlatformGroupId}:${quarkusPlatformArtifactId}:${quarkusPlatformVersion}"
        )
    )
    implementation(
        enforcedPlatform(
            "${quarkusPlatformGroupId}:quarkus-operator-sdk-bom:${quarkusPlatformVersion}"
        )
    )

    implementation("io.quarkus:quarkus-arc")
    implementation("io.quarkus:quarkus-container-image-docker")
    implementation("io.quarkus:quarkus-kubernetes")
    implementation("io.quarkus:quarkus-kubernetes-client")
    implementation("io.quarkiverse.operatorsdk:quarkus-operator-sdk-bundle-generator")
    implementation("io.quarkiverse.operatorsdk:quarkus-operator-sdk")
    implementation("com.hedera.hashgraph:sdk:${hederaSdkVersion}")

    compileOnly("io.sundr:builder-annotations:${sundrVersion}")
    compileOnly("org.projectlombok:lombok:${lombokVersion}")

    testImplementation("io.quarkus:quarkus-junit5")
    testImplementation("io.quarkus:quarkus-jacoco")
    testImplementation("org.awaitility:awaitility")
    testImplementation("org.assertj:assertj-core:${assertjVersion}")
    testImplementation("io.grpc:grpc-netty")
    testImplementation("com.hedera.fullstack:fullstack-helm-client:0.28.3")
    componentTestImplementation(
        "io.javaoperatorsdk:operator-framework-junit-5:${javaoperatorsdkVersion}"
    )
    componentTestImplementation("org.testcontainers", "junit-jupiter")
    componentTestImplementation("com.dajudge.kindcontainer", "kindcontainer")

    testFixturesImplementation(
        enforcedPlatform(
            "${quarkusPlatformGroupId}:${quarkusPlatformArtifactId}:${quarkusPlatformVersion}"
        )
    )

    testFixturesImplementation(
        enforcedPlatform(
            "${quarkusPlatformGroupId}:quarkus-operator-sdk-bom:${quarkusPlatformVersion}"
        )
    )

    testFixturesImplementation("io.fabric8:kubernetes-client")
    testFixturesImplementation("org.awaitility:awaitility")
}

objectModelGenerator {
    register(sourceSets.main) {
        configBuilderOverrides =
            mapOf(
                "common.operator.solo.hedera.com-v1.yml" to
                    defaultConfigBuilder
                        .build()
                        .toBuilder()
                        .packageOverrides(
                            mapOf(
                                "com.hedera.solo.operator.v1alpha1" to
                                    "com.hedera.solo.operator.model"
                            )
                        )
            )
        removeFiles =
            setOf(
                file("com/hedera/solo/operator/model/Common.java"),
                file("com/hedera/solo/operator/model/CommonSpec.java"),
            )
    }
    register(sourceSets.componentTest)
}
