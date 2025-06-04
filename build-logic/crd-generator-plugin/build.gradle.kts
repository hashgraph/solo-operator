// SPDX-License-Identifier: Apache-2.0

plugins {
    `kotlin-dsl`
    `java-gradle-plugin`
}

repositories {
    // Use Maven Central for resolving dependencies.
    maven { url = uri("https://artifacts.swirldslabs.io/artifactory/central-maven-external/") }
}

val fabric8Version = libs.versions.fabric8.get()

dependencies {
    // Use JUnit Jupiter for testing.
    testImplementation(libs.junit.jupiter)

    implementation("io.fabric8:kubernetes-model-core:${fabric8Version}")
    implementation("io.fabric8:crd-generator-collector:${fabric8Version}")
    implementation("com.hedera.solo.operator:shared-lib")

    testRuntimeOnly("org.junit.platform:junit-platform-launcher")
}

gradlePlugin {
    // Define the plugin
    val crdGenerator by
        plugins.creating {
            id = "com.hedera.solo.operator.buildlogic.crdGenerator"
            implementationClass =
                "com.hedera.solo.operator.buildlogic.crdgenerator.CrdGeneratorPlugin"
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
