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
    jacoco
}

tasks.jacocoTestReport {
    //    sourceSets(sourceSets.main.get(), sourceSets.componentTest.get())
    reports {
        html.required = true
        xml.required = true
    }
}

/*
tasks.test {
    finalizedBy(tasks.jacocoTestReport) // report is always generated after tests run
    extensions.configure<JacocoTaskExtension>("jacoco") {
        excludeClassLoaders = listOf("*QuarkusClassLoader")
//        setDestinationFile(layout.buildDirectory.file("jacoco-quarkus.exec").get().asFile)
//        exclude("com.hedera.solo.operator.test.fixtures.Finalizers")
//        exclude("com.hedera.solo.operator.test.fixtures.*")
//        exclude("com.hedera.solo.operator.test.fixtures.**")
//        setTestNameIncludePatterns(listOf("com.hedera.solo.operator.test.fixtures.Finalizers"))
//        enabled = false
//        reports {
//            require(true)
//        }
    }
//    extensions.configure<JacocoExtens> {
//        exclude("com.hedera.solo.operator.test.fixtures.*")
//    }
}
*/
tasks.componentTest {
    finalizedBy(tasks.named("jacocoComponenetTestReport"))
    //    finalizedBy(tasks.jacocoTestReport)
    extensions.configure<JacocoTaskExtension> {
        setDestinationFile(
            layout.buildDirectory.file("jacoco/jacoco-component-test.exec").get().asFile
        )
    }
    //    extensions.configure<ReportingExtension> {
    //        reports.named("jacoco") {
    //
    //        }
    //    }
}

tasks.register("jacocoComponenetTestReport", JacocoReport::class) {
    //    shouldRunAfter(tasks.componentTest)
    sourceSets(sourceSets.main.get())
    dependsOn(tasks.componentTest)
    enabled = true
    executionData(layout.buildDirectory.file("jacoco/jacoco-component-test.exec").get().asFile)
    reports.xml.required = true
    reports.xml.outputLocation =
        layout.buildDirectory.file("jacoco/jacoco-component-test.xml").get().asFile
}

// tasks.jacocoTestReport {
////    dependsOn(tasks.test) // tests are required to run before generating the report
////    dependsOn(tasks.componentTest)
////    executionData = files(fileTree(layout.buildDirectory).include("/jacoco/*.exec"))
////    println("executionData: " + executionData.asPath)
////    enabled = true
//    reports.xml.required = true
////    reports.xml.outputLocation = file(executionData.asPath.plus(".xml"))
////    println("project.gradle.startParameter.taskNames: " +
// project.gradle.startParameter.taskNames)
////    if (project.gradle.startParameter.taskNames.stream().anyMatch("componentTest"::equals)) {
////        reports.xml.outputLocation =
// project.layout.buildDirectory.file("jacoco-report/componentTest-jacoco.xml")
////    }
////    if (project.hasProperty("jacocoTestReport.reports.xml.outputLocation")) {
////        reports.xml.outputLocation =
// file(project.property("jacocoTestReport.reports.xml.outputLocation").toString())
////    }
////    println("reports.xml.outputLocation: " + reports.xml.outputLocation.get())
// }

// tasks.jacocoTestReport {
//    afterEvaluate {
//        classDirectories.setFrom(files(classDirectories.files.stream()
//            .filter { file -> !file.absolutePath.contains("testFixtures") } //
// build/classes/java/testFixtures
//        ))
//    }
// }

// tasks.test {
//    finalizedBy(tasks.) jacocoTestReport
//            jacoco {
//                excludeClassLoaders = ["*QuarkusClassLoader"]
//                destinationFile = layout.buildDirectory.file("jacoco-quarkus.exec").get().asFile
//            }
//    jacocoTestReport.enabled = false
// }

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

configurations.named("componentTestImplementation").configure {
    extendsFrom(configurations.named("testImplementation").get())
}

configurations.named("componentTestAnnotationProcessor").configure {
    extendsFrom(configurations.named("annotationProcessor").get())
}

configurations.named("componentTestCompileOnly").configure {
    extendsFrom(configurations.named("compileOnly").get())
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

/*
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

//                implementation(project())
//                implementation(
//                    enforcedPlatform(
//                        "${quarkusPlatformGroupId}:${quarkusPlatformArtifactId}:${quarkusPlatformVersion}"
//                    )
//                )
//                implementation(
//                    enforcedPlatform(
//                        "${quarkusPlatformGroupId}:quarkus-operator-sdk-bom:${quarkusPlatformVersion}"
//                    )
//                )
//                implementation("org.testcontainers:junit-jupiter")
//                implementation("com.dajudge.kindcontainer:kindcontainer")
//                implementation("io.quarkus:quarkus-kubernetes-client")
//
////                implementation("io.quarkus:quarkus-arc")
//                implementation("io.quarkus:quarkus-junit5")
//                implementation("io.quarkus:quarkus-jacoco")
//
//                implementation("io.quarkiverse.operatorsdk:quarkus-operator-sdk")
//                implementation(libs.io.javaoperatorsdk.operator.framework.junit.v5)
//                implementation("org.jboss.slf4j:slf4j-jboss-logmanager")
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
*/

tasks.named("assemble") { dependsOn(tasks.named("compileComponentTestJava")) }

tasks.named("check") { dependsOn(testing.suites.named("componentTest")) }

/*
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
*/

sourceSets.forEach() {
    println(it)
    //    it.allSource.forEach() {
    //        println(it)
    //    }
}
