// SPDX-License-Identifier: Apache-2.0

plugins {
    `java-library`
    `kotlin-dsl`
    `java-gradle-plugin`
}

group = "com.hedera.solo.operator.buildlogic"

repositories {
    // Source Maven Central Artifacts from JFrog Artifactory
    maven { url = uri("https://artifacts.hashgraph.io/artifactory/central-maven-external/") }
}

dependencies {
    api(libs.fabric8.crd.generator.api.v2)
    testImplementation(libs.junit.jupiter)
}

gradlePlugin {
    val sharedLib by
        plugins.creating {
            id = "${group}.sharedLib"
            implementationClass = "${group}.sharedlib.SharedLibPlugin"
        }
}
