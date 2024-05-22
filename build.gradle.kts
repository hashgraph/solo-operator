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
    id("com.hedera.solo.operator.objectModelGenerator")
}

val quarkusPlatformGroupId: String by project
val quarkusPlatformArtifactId: String by project
val quarkusPlatformVersion: String by project
val assertjVersion: String by project
val sundrVersion: String by project
val fabric8Version: String by project
val lombokVersion: String by project

dependencies {
    annotationProcessor("io.sundr:builder-annotations:${sundrVersion}")
    annotationProcessor("io.fabric8:kubernetes-client:${fabric8Version}")
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

    compileOnly("io.sundr:builder-annotations:${sundrVersion}")
    compileOnly("org.projectlombok:lombok:${lombokVersion}")

    testImplementation("io.quarkus:quarkus-junit5")
    testImplementation("io.quarkus:quarkus-jacoco")
    testImplementation("org.awaitility:awaitility")
    testImplementation("org.assertj:assertj-core:${assertjVersion}")

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
    testFixturesImplementation("org.assertj:assertj-core:${assertjVersion}")
}

objectModelGenerator {
    defaultConfigBuilder.uppercaseEnums(false)
    defaultConfigBuilder.objectExtraAnnotations(true)
    defaultConfigBuilder.packageOverrides(
        mapOf("com.hedera.solo.operator.v1alpha1" to "com.hedera.solo.operator.model.v1alpha1")
    )
    configBuilderOverrides =
        mapOf(
            "common.operator.solo.hedera.com-v1.yml" to
                defaultConfigBuilder
                    .build()
                    .toBuilder()
                    .packageOverrides(
                        mapOf(
                            "com.hedera.solo.operator.v1alpha1" to "com.hedera.solo.operator.model"
                        )
                    )
        )
    removeFiles =
        setOf(
            file("com/hedera/solo/operator/model/Common.java"),
            file("com/hedera/solo/operator/model/CommonSpec.java"),
        )
}

configurations.all {
    resolutionStrategy {
        force("io.fabric8:crd-generator-api:6.11.0")
        force("io.fabric8:generator-annotations:6.11.0")
        force("io.fabric8:knative-client:6.11.0")
        force("io.fabric8:knative-model:6.11.0")
        force("io.fabric8:kubernetes-client:6.11.0")
        force("io.fabric8:kubernetes-client-api:6.11.0")
        force("io.fabric8:kubernetes-httpclient-okhttp:6.11.0")
        force("io.fabric8:kubernetes-httpclient-vertx:6.11.0")
        force("io.fabric8:kubernetes-model-admissionregistration:6.11.0")
        force("io.fabric8:kubernetes-model-apiextensions:6.11.0")
        force("io.fabric8:kubernetes-model-apps:6.11.0")
        force("io.fabric8:kubernetes-model-autoscaling:6.11.0")
        force("io.fabric8:kubernetes-model-batch:6.11.0")
        force("io.fabric8:kubernetes-model-certificates:6.11.0")
        force("io.fabric8:kubernetes-model-common:6.11.0")
        force("io.fabric8:kubernetes-model-coordination:6.11.0")
        force("io.fabric8:kubernetes-model-core:6.11.0")
        force("io.fabric8:kubernetes-model-discovery:6.11.0")
        force("io.fabric8:kubernetes-model-events:6.11.0")
        force("io.fabric8:kubernetes-model-extensions:6.11.0")
        force("io.fabric8:kubernetes-model-flowcontrol:6.11.0")
        force("io.fabric8:kubernetes-model-gatewayapi:6.11.0")
        force("io.fabric8:kubernetes-model-metrics:6.11.0")
        force("io.fabric8:kubernetes-model-networking:6.11.0")
        force("io.fabric8:kubernetes-model-node:6.11.0")
        force("io.fabric8:kubernetes-model-policy:6.11.0")
        force("io.fabric8:kubernetes-model-rbac:6.11.0")
        force("io.fabric8:kubernetes-model-resource:6.11.0")
        force("io.fabric8:kubernetes-model-scheduling:6.11.0")
        force("io.fabric8:kubernetes-model-storageclass:6.11.0")
        force("io.fabric8:openshift-client:6.11.0")
        force("io.fabric8:openshift-client-api:6.11.0")
        force("io.fabric8:openshift-model:6.11.0")
        force("io.fabric8:openshift-model-clusterautoscaling:6.11.0")
        force("io.fabric8:openshift-model-config:6.11.0")
        force("io.fabric8:openshift-model-console:6.11.0")
        force("io.fabric8:openshift-model-hive:6.11.0")
        force("io.fabric8:openshift-model-installer:6.11.0")
        force("io.fabric8:openshift-model-machine:6.11.0")
        force("io.fabric8:openshift-model-machineconfig:6.11.0")
        force("io.fabric8:openshift-model-miscellaneous:6.11.0")
        force("io.fabric8:openshift-model-monitoring:6.11.0")
        force("io.fabric8:openshift-model-operator:6.11.0")
        force("io.fabric8:openshift-model-operatorhub:6.11.0")
        force("io.fabric8:openshift-model-storageversionmigrator:6.11.0")
        force("io.fabric8:openshift-model-tuned:6.11.0")
        force("io.fabric8:openshift-model-whereabouts:6.11.0")
    }
}
