import com.fasterxml.jackson.databind.ObjectMapper
import com.fasterxml.jackson.dataformat.yaml.YAMLFactory
import org.yaml.snakeyaml.Yaml

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
val wiremockVersion: String by project
val smallryeCertificateGeneratorVersion: String by project

dependencies {
    annotationProcessor("io.sundr:builder-annotations:${sundrVersion}")
    annotationProcessor("io.fabric8:kubernetes-client:${fabric8Version}")
    annotationProcessor("org.projectlombok:lombok:${lombokVersion}")
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
    implementation("io.quarkus:quarkus-security")
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
    testImplementation("org.wiremock:wiremock:${wiremockVersion}")
    testImplementation(
        "io.smallrye.certs:smallrye-certificate-generator-junit5:${smallryeCertificateGeneratorVersion}"
    )
    testImplementation("io.fabric8:crd-generator-api-v2:${fabric8Version}")
    testImplementation("io.fabric8:crd-generator-collector:${fabric8Version}")
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
    testFixturesImplementation(
        "io.smallrye.certs:smallrye-certificate-generator-junit5:${smallryeCertificateGeneratorVersion}"
    )
    testFixturesImplementation("org.wiremock:wiremock:${wiremockVersion}")
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

// Require JOSDK 5.0.3 until a newer version of quarkus using JOSDK 5.0.3 or later is available.
configurations.all {
    resolutionStrategy {
        force("io.javaoperatorsdk:micrometer-support:${javaoperatorsdkVersion}")
        force("io.javaoperatorsdk:operator-framework-core:${javaoperatorsdkVersion}")
        force("io.javaoperatorsdk:operator-framework-junit-5:${javaoperatorsdkVersion}")
        force("io.fabric8:crd-generator-api:${fabric8Version}")
        force("io.fabric8:crd-generator-api-v2:${fabric8Version}")
        force("io.fabric8:crd-generator-collector:${fabric8Version}")
        force("io.fabric8:generator-annotations:${fabric8Version}")
        force("io.fabric8:knative-client:${fabric8Version}")
        force("io.fabric8:knative-model:${fabric8Version}")
        force("io.fabric8:kubernetes-client:${fabric8Version}")
        force("io.fabric8:kubernetes-client-api:${fabric8Version}")
        force("io.fabric8:kubernetes-client-bom:${fabric8Version}")
        force("io.fabric8:kubernetes-httpclient-vertx:${fabric8Version}")
        force("io.fabric8:kubernetes-model-admissionregistration:${fabric8Version}")
        force("io.fabric8:kubernetes-model-apiextensions:${fabric8Version}")
        force("io.fabric8:kubernetes-model-apps:${fabric8Version}")
        force("io.fabric8:kubernetes-model-autoscaling:${fabric8Version}")
        force("io.fabric8:kubernetes-model-batch:${fabric8Version}")
        force("io.fabric8:kubernetes-model-certificates:${fabric8Version}")
        force("io.fabric8:kubernetes-model-common:${fabric8Version}")
        force("io.fabric8:kubernetes-model-coordination:${fabric8Version}")
        force("io.fabric8:kubernetes-model-core:${fabric8Version}")
        force("io.fabric8:kubernetes-model-discovery:${fabric8Version}")
        force("io.fabric8:kubernetes-model-events:${fabric8Version}")
        force("io.fabric8:kubernetes-model-extensions:${fabric8Version}")
        force("io.fabric8:kubernetes-model-flowcontrol:${fabric8Version}")
        force("io.fabric8:kubernetes-model-gatewayapi:${fabric8Version}")
        force("io.fabric8:kubernetes-model-metrics:${fabric8Version}")
        force("io.fabric8:kubernetes-model-networking:${fabric8Version}")
        force("io.fabric8:kubernetes-model-node:${fabric8Version}")
        force("io.fabric8:kubernetes-model-policy:${fabric8Version}")
        force("io.fabric8:kubernetes-model-rbac:${fabric8Version}")
        force("io.fabric8:kubernetes-model-resource:${fabric8Version}")
        force("io.fabric8:kubernetes-model-scheduling:${fabric8Version}")
        force("io.fabric8:kubernetes-model-storageclass:${fabric8Version}")
        force("io.fabric8:maven-model-helper:37:${fabric8Version}")
        force("io.fabric8:openshift-client:${fabric8Version}")
        force("io.fabric8:openshift-client-api:${fabric8Version}")
        force("io.fabric8:openshift-model:${fabric8Version}")
        force("io.fabric8:openshift-model-autoscaling:${fabric8Version}")
        force("io.fabric8:openshift-model-config:${fabric8Version}")
        force("io.fabric8:openshift-model-console:${fabric8Version}")
        force("io.fabric8:openshift-model-hive:${fabric8Version}")
        force("io.fabric8:openshift-model-installer:${fabric8Version}")
        force("io.fabric8:openshift-model-machine:${fabric8Version}")
        force("io.fabric8:openshift-model-machineconfiguration:${fabric8Version}")
        force("io.fabric8:openshift-model-miscellaneous:${fabric8Version}")
        force("io.fabric8:openshift-model-monitoring:${fabric8Version}")
        force("io.fabric8:openshift-model-operator:${fabric8Version}")
        force("io.fabric8:openshift-model-operatorhub:${fabric8Version}")
        force("io.fabric8:openshift-model-storageversionmigrator:${fabric8Version}")
        force("io.fabric8:openshift-model-tuned:${fabric8Version}")
        force("io.fabric8:openshift-model-whereabouts:${fabric8Version}")
        force("io.fabric8:zjsonpatch:${fabric8Version}")
    }
}

tasks.register("modifyCRD") {
    doLast {
        var mapper = ObjectMapper(YAMLFactory())
        var f = project.layout.buildDirectory.dir("kubernetes").get().asFile.listFiles()
        for (ff in f!!) {
            var orig = project.layout.buildDirectory.dir("generated/object-model-generator/main/crd").get().file(ff.name).asFile
            var m1 = mapper.readValue(orig, Map::class.java)
            var m2 = mapper.readValue(ff, Map::class.java)
            println(ff.absolutePath)
        }
    }
}

tasks.getByName("build").dependsOn("modifyCRD")
tasks.getByName("modifyCRD").dependsOn("quarkusBuild")
