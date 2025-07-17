// SPDX-License-Identifier: Apache-2.0

plugins {
    `kotlin-dsl`
    `java-gradle-plugin`
}

group = "com.hedera.solo.operator.buildlogic"

repositories {
    // Use Maven Central for resolving dependencies.
    maven { url = uri("https://artifacts.hashgraph.io/artifactory/central-maven-external/") }
}

dependencies {
    implementation(libs.kubernetes.model.core)
    implementation(libs.crd.generator.collector)
    implementation(libs.fabric8.crd.generator.api.v2)

    testImplementation(libs.junit.jupiter)

    testRuntimeOnly(libs.junit.platform.launcher)
}

gradlePlugin {
    // Define the plugin
    val crdGenerator by
        plugins.creating {
            id = "${group}.crdGenerator"
            implementationClass = "${group}.crdgenerator.CrdGeneratorPlugin"
        }
}

// Add a source set for the functional test suite
val functionalTestSourceSet = sourceSets.create("functionalTest") {}

configurations["functionalTestImplementation"].extendsFrom(configurations["testImplementation"])

configurations["functionalTestRuntimeOnly"].extendsFrom(configurations["testRuntimeOnly"])

// Add a task to run the functional tests
val functionalTest by
    tasks.registering(Test::class) {
        testClassesDirs = functionalTestSourceSet.output.classesDirs
        classpath = functionalTestSourceSet.runtimeClasspath
        useJUnitPlatform()
    }

gradlePlugin.testSourceSets.add(functionalTestSourceSet)

tasks.named<Task>("check") {
    // Run the functional tests as part of `check`
    dependsOn(functionalTest)
}

tasks.named<Test>("test") {
    // Use JUnit Jupiter for unit tests.
    useJUnitPlatform()
}
