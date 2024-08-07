import com.hedera.solo.operator.buildlogic.objectmodelgenerator.ObjectModelGeneratorPlugin

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

dependencies {
    annotationProcessor("io.sundr:builder-annotations:${sundrVersion}")
    annotationProcessor("io.fabric8:kubernetes-client:${fabric8Version}")
    annotationProcessor(libs.io.fabric8.crd.generator.apt)
    testAnnotationProcessor("io.sundr:builder-annotations:${sundrVersion}")
    testAnnotationProcessor("io.fabric8:kubernetes-client:${fabric8Version}")
    testAnnotationProcessor(libs.io.fabric8.crd.generator.apt)
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
//    implementation("org.bouncycastle:bcprov-jdk15on:1.70")
    implementation("org.bouncycastle:bcpkix-jdk18on")

    compileOnly("io.sundr:builder-annotations:${sundrVersion}")
    compileOnly("org.projectlombok:lombok:${lombokVersion}")
    testCompileOnly("io.sundr:builder-annotations:${sundrVersion}")
    testCompileOnly("org.projectlombok:lombok:${lombokVersion}")

    testImplementation("io.quarkus:quarkus-junit5")
    testImplementation("io.quarkus:quarkus-jacoco")
    testImplementation("org.awaitility:awaitility")
    testImplementation("org.assertj:assertj-core:${assertjVersion}")
    testImplementation(libs.io.javaoperatorsdk.operator.framework.junit.v5)
//    testImplementation("org.testcontainers", "testcontainers")
    testImplementation("org.testcontainers", "junit-jupiter")
    testImplementation("com.dajudge.kindcontainer", "kindcontainer")

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
    defaultConfigBuilder.generatedAnnotations(false)
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

testing {
    suites {
//        val test by getting(JvmTestSuite::class) {
//            useJUnitJupiter()
//        }

        named<JvmTestSuite>("componentTest") {
//        register<JvmTestSuite>("componentTest") {
//            plugins {
//                id("java")
//                id("com.hedera.solo.operator.buildlogic.objectModelGenerator")
//            }
            dependencies {
                annotationProcessor("io.sundr:builder-annotations:${sundrVersion}")
                annotationProcessor("io.fabric8:kubernetes-client:${fabric8Version}")
                annotationProcessor(libs.io.fabric8.crd.generator.apt)

                compileOnly("io.sundr:builder-annotations:${sundrVersion}")
                compileOnly("org.projectlombok:lombok:${lombokVersion}")

                implementation(project())
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
                implementation("org.testcontainers:junit-jupiter")
                implementation("com.dajudge.kindcontainer:kindcontainer")
                implementation("io.quarkus:quarkus-kubernetes-client")

//                implementation("io.quarkus:quarkus-arc")
                implementation("io.quarkus:quarkus-junit5")
                implementation("io.quarkus:quarkus-jacoco")

                implementation("io.quarkiverse.operatorsdk:quarkus-operator-sdk")
                implementation(libs.io.javaoperatorsdk.operator.framework.junit.v5)
                implementation("org.jboss.slf4j:slf4j-jboss-logmanager")
            }

//            apply(plugin = "java")
////            apply(plugin = "com.hedera.solo.operator.buildlogic.objectModelGenerator")
//            apply<ObjectModelGeneratorPlugin>()
//            objectModelGenerator {}
//            targets {
//                all {
//                    testTask.configure {
////                        systemProperty("java.util.logging.manager", "org.jboss.logmanager.LogManager")
////                        systemProperty("quarkus.log.level", "INFO")
////                        systemProperty("java.util.logging.config.file", "${projectDir}/src/componentTest/resources/logging.properties")
//
////                        shouldRunAfter(testing.suites.named("test"))
//                    }
//                }
//            }
        }
    }
}

tasks.named("assemble") {
    dependsOn(tasks.named("compileComponentTestJava"))
}

tasks.named("check") {
    dependsOn(testing.suites.named("componentTest"))
}

sourceSets {
    create("intTest") {
        compileClasspath += sourceSets.main.get().output
        runtimeClasspath += sourceSets.main.get().output
    }
}

val intTestImplementation by configurations.getting {
    extendsFrom(configurations.implementation.get())
}
val intTestRuntimeOnly by configurations.getting

configurations["intTestRuntimeOnly"].extendsFrom(configurations.runtimeOnly.get())

val intTest = task<Test>("intTest") {
    description = "Runs integration tests."
    group = "verification"

    testClassesDirs = sourceSets["intTest"].output.classesDirs
    classpath = sourceSets["intTest"].runtimeClasspath
    shouldRunAfter("test")

    useJUnitPlatform()

    testLogging {
        events("passed")
    }
}

tasks.check { dependsOn(intTest) }

sourceSets.forEach() {
    println(it)
//    it.allSource.forEach() {
//        println(it)
//    }
}
