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
    alias(libs.plugins.quarkus)
    id("com.hedera.solo.operator.objectModelGenerator")
}

dependencies {
    annotationProcessor(libs.sundr.builder.annotations)
    annotationProcessor(libs.fabric8.k8s.client)
    implementation("io.fabric8", "generator-annotations")

    implementation(enforcedPlatform(libs.quarkus.platform))
    implementation(enforcedPlatform(libs.quarkus.operator.sdk))
    implementation(platform(libs.java.operator.sdk))
    implementation(platform(libs.fabric8.k8s.client.bom))

    implementation("io.quarkus:quarkus-arc")
    implementation("io.quarkus:quarkus-container-image-docker")
    implementation("io.quarkus:quarkus-kubernetes")
    implementation("io.quarkus:quarkus-kubernetes-client")
    implementation("io.quarkiverse.operatorsdk:quarkus-operator-sdk-bundle-generator")
    implementation("io.quarkiverse.operatorsdk:quarkus-operator-sdk")

    compileOnly(libs.sundr.builder.annotations)
    compileOnly(libs.lombok)

    testImplementation("io.quarkus:quarkus-junit5")
    testImplementation("io.quarkus:quarkus-jacoco")
    testImplementation("org.awaitility:awaitility")
    testImplementation(libs.assertj)

    testFixturesImplementation(enforcedPlatform(libs.quarkus.platform))
    testFixturesImplementation(enforcedPlatform(libs.quarkus.operator.sdk))

    testFixturesImplementation("io.fabric8:kubernetes-client")
    testFixturesImplementation("org.awaitility:awaitility")
    testFixturesImplementation(libs.assertj)
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
