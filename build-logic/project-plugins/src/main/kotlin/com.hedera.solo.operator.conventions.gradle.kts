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
    id("java")
    id("java-test-fixtures")
    id("com.hedera.solo.operator.spotless-conventions")
    id("com.hedera.solo.operator.spotless-java-conventions")
    id("com.hedera.solo.operator.spotless-kotlin-conventions")
}

repositories {
    // Source Maven Central Artifacts from JFrog Artifactory
    maven { url = uri("https://artifacts.swirldslabs.io/artifactory/central-maven-external/") }
    maven {
        url = uri("https://artifacts.swirldslabs.io/artifactory/central-maven-snapshot-external/")
    }
}

java {
    sourceCompatibility = JavaVersion.VERSION_21
    targetCompatibility = JavaVersion.VERSION_21

    toolchain {
        languageVersion.set(JavaLanguageVersion.of(21))
        vendor.set(JvmVendorSpec.ADOPTIUM)
    }
}

tasks.withType<Test>().configureEach {
    systemProperty("java.util.logging.manager", "org.jboss.logmanager.LogManager")
}

tasks.withType<JavaCompile>().configureEach {
    options.encoding = "UTF-8"
    options.compilerArgs.add("-parameters")
}

tasks.withType<Javadoc>().configureEach {
    options.encoding = "UTF-8"

    // Configure Javadoc Tag Support
    (options as StandardJavadocDocletOptions).tags(
        "apiNote:a:API Note:",
        "implSpec:a:Implementation Requirements:",
        "implNote:a:Implementation Note:"
    )
}

// Ensure that all JAR files are reproducible
tasks.withType<Jar>().configureEach {
    isReproducibleFileOrder = true
    isPreserveFileTimestamps = false
    fileMode = 664
    dirMode = 775
}

// Ensure JaCoCo coverage is generated and aggregated
// tasks.jacocoTestReport.configure {
//    reports {
//        xml.required.set(true)
//        html.required.set(true)
//    }
//
//    val testExtension = tasks.test.get().extensions.getByType<JacocoTaskExtension>()
//    executionData.setFrom(testExtension.destinationFile)
//
//    shouldRunAfter(tasks.named("check"))
// }

// Ensure the check task also runs the JaCoCo coverage report
// tasks.named("check").configure { dependsOn(tasks.named<JacocoReport>("jacocoTestReport")) }

tasks.named("assemble").configure { dependsOn(tasks.named("testClasses")) }

tasks.check { dependsOn(gradle.includedBuild("build-logic").task(":check")) }

tasks.test { dependsOn(gradle.includedBuild("build-logic").task(":test")) }
