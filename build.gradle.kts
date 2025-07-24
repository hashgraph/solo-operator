// SPDX-License-Identifier: Apache-2.0

import java.nio.file.Path

plugins {
    id("com.hedera.solo.operator.conventions")
    id("com.hedera.solo.operator.versioning")
    alias(libs.plugins.quarkus)
    id("com.hedera.solo.operator.buildlogic.objectModelGenerator")
    id("com.hedera.solo.operator.buildlogic.gitExtractor")
    id("com.hedera.solo.operator.buildlogic.crdGenerator")
}

dependencies {
    annotationProcessor(libs.sundr.builder.annotations)
    annotationProcessor(libs.fabric8.kubernetes.client)
    annotationProcessor(libs.lombok)

    implementation(libs.fabric8.generator.annotations)
    implementation(libs.fabric8.crd.generator.api.v2)

    implementation(enforcedPlatform(libs.quarkus.bom))
    implementation(enforcedPlatform(libs.quarkus.operator.sdk.bom))

    implementation(libs.bundles.quarkus.platform)
    implementation(libs.quarkus.operator.sdk.bundle)
    implementation(libs.quarkus.operator.sdk)
    implementation(libs.hedera.sdk)

    compileOnly(libs.sundr.builder.annotations)
    compileOnly(libs.lombok)
    compileOnly(libs.spotbugs.annotations)

    testImplementation(libs.bundles.quarkus.junit5.platform)
    testImplementation(libs.awaitility)
    testImplementation(libs.assertj.core)
    testImplementation(libs.grpc.netty)
    testImplementation(libs.fullstack.helm.client)
    testImplementation(libs.wiremock)
    testImplementation(libs.smallrye.cert.junit)

    componentTestImplementation(libs.javaoperatorsdk.junit5)
    componentTestImplementation(libs.testcontainers.junit)
    componentTestImplementation(libs.kindcontainer)

    testFixturesImplementation(enforcedPlatform(libs.quarkus.bom))
    testFixturesImplementation(enforcedPlatform(libs.quarkus.operator.sdk.bom))

    testFixturesImplementation(libs.fabric8.kubernetes.client)
    testFixturesImplementation(libs.awaitility)
    testFixturesImplementation(libs.assertj.core)
    testFixturesImplementation(libs.junit.jupiter.api)
    testFixturesImplementation(libs.microprofile.config.api)
    testFixturesImplementation(libs.hedera.sdk)
    testFixturesImplementation(libs.smallrye.cert.junit)
    testFixturesImplementation(libs.wiremock)
    testFixturesImplementation(libs.fullstack.helm.client)
}

gitExtractor {
    register(sourceSets.test) {
        addResources(
            "https://github.com/hashgraph/hedera-services",
            "refs/tags/v0.56.5",
            "hedera-node/configuration/mainnet",
            "import/hedera/mainnet",
        )
    }
    register(sourceSets.componentTest) {
        addResources(
            "https://github.com/hashgraph/hedera-services",
            "refs/tags/v0.56.5",
            "hedera-node/configuration/mainnet",
            "import/hedera/mainnet",
        )
    }
}

objectModelGenerator {
    register(sourceSets.main) {
        configBuilderOverrides =
            mapOf(
                "commons.operator.solo.hedera.com-v1.yml" to
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

crdGenerator {
    register(sourceSets.componentTest) {
        sourceSet.resources { srcDirs(outputDir) }
        outputDir = File(outputDir, "META-INF/fabric8")
        postProcessorClassName =
            "com.hedera.solo.operator.library.crdpostprocessor.SharedCrdPostProcessor"
        postProcessorClassPath = sourceSets.main.get().runtimeClasspath
        postProcessorNewInstance = { clazz ->
            clazz
                .getConstructor(Path::class.java)
                .newInstance(
                    layout.buildDirectory
                        .dir("generated/object-model-generator/${sourceSet.name}/crd")
                        .get()
                        .asFile
                        .toPath()
                )
        }
    }
}

tasks.register<Copy>("copyAdditionalComponentTestCrdResources") {
    dependsOn("quarkusBuild")
    dependsOn("generateComponentTestCrd")
    from(layout.buildDirectory.dir("kubernetes"))
    into(layout.buildDirectory.dir("generated/crd-generator/componentTest/META-INF/fabric8"))
}

tasks.named("processComponentTestResources") {
    dependsOn("copyAdditionalComponentTestCrdResources")
}

quarkus { buildForkOptions { jvmArgs("-Dproject.root=${rootProject.projectDir.absolutePath}") } }
