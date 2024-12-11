// SPDX-License-Identifier: Apache-2.0

plugins {
    id("com.hedera.solo.operator.conventions")
    id("com.hedera.solo.operator.versioning")
    id("io.quarkus")
    id("com.hedera.solo.operator.buildlogic.objectModelGenerator")
    id("com.hedera.solo.operator.buildlogic.gitExtractor")
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
val spotbugsVersion: String by project

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
    implementation("io.quarkus:quarkus-resteasy-qute:${quarkusPlatformVersion}")
    implementation("com.hedera.hashgraph:sdk:${hederaSdkVersion}")

    compileOnly("io.sundr:builder-annotations:${sundrVersion}")
    compileOnly("org.projectlombok:lombok:${lombokVersion}")
    compileOnly("com.github.spotbugs:spotbugs-annotations:${spotbugsVersion}")

    testImplementation("io.quarkus:quarkus-junit5")
    testImplementation("io.quarkus:quarkus-junit5-mockito")
    testImplementation("io.quarkus:quarkus-jacoco")
    testImplementation("org.awaitility:awaitility")
    testImplementation("org.assertj:assertj-core:${assertjVersion}")
    testImplementation("io.grpc:grpc-netty")
    testImplementation("com.hedera.fullstack:fullstack-helm-client:0.32.0")
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
    testFixturesImplementation("org.assertj:assertj-core:${assertjVersion}")
    testFixturesImplementation("org.junit.jupiter:junit-jupiter-api")
    testFixturesImplementation("org.eclipse.microprofile.config:microprofile-config-api")
    testFixturesImplementation("com.hedera.hashgraph:sdk:${hederaSdkVersion}")
}

//gitExtractor {
//    addResources(
//        "https://github.com/hashgraph/hedera-services",
//        "refs/tags/v0.56.5",
//        "hedera-node/configuration/mainnet",
//        "import/hedera/mainnet"
//    )
//}

gitExtractor {
    register(sourceSets.test) {
        addResources(
            "https://github.com/hashgraph/hedera-services",
            "refs/tags/v0.56.5",
            "hedera-node/configuration/mainnet",
            "import/hedera/mainnet"
        )
    }
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
