// SPDX-License-Identifier: Apache-2.0

plugins {
    id("java")
    id("java-test-fixtures")
    id("com.hedera.solo.operator.component-test")
    id("com.hedera.solo.operator.checkstyle-conventions")
    id("com.hedera.solo.operator.spotless-conventions")
    id("com.hedera.solo.operator.spotless-java-conventions")
    id("com.hedera.solo.operator.spotless-kotlin-conventions")
    jacoco
    if (!System.getenv().containsKey("CI")) {
        idea
    }
}

repositories {
    // Source Maven Central Artifacts from JFrog Artifactory
    maven { url = uri("https://artifacts.hashgraph.io/artifactory/central-maven-external/") }
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
    if (System.getenv().containsKey("CI")) {
        environment("SOLO_TEST_SNAPSHOT_ENABLED", "true")
        jvmArgs("-XX:+HeapDumpOnOutOfMemoryError")
    }
    systemProperty("java.util.logging.manager", "org.jboss.logmanager.LogManager")
    systemProperty("junit.jupiter.execution.parallel.enabled", "false")
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
        "implNote:a:Implementation Note:",
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

tasks {
    test {
        finalizedBy(jacocoTestReport)
        configure<JacocoTaskExtension> {
            excludes = listOf("*QuarkusClassLoader")
            setDestinationFile(layout.buildDirectory.file("jacoco-quarkus.exec").get().asFile)
        }
    }
    jacocoTestReport { enabled = false }
}

tasks.named("assemble").configure { dependsOn(tasks.named("testClasses")) }

tasks.check { dependsOn(gradle.includedBuild("build-logic").task(":check")) }

tasks.test { dependsOn(gradle.includedBuild("build-logic").task(":test")) }
