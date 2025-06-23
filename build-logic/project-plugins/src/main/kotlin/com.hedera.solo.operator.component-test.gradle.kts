// SPDX-License-Identifier: Apache-2.0

plugins {
    java
    jacoco
}

testing { suites { register<JvmTestSuite>("componentTest") { useJUnitJupiter() } } }

configurations.named("componentTestAnnotationProcessor").configure {
    extendsFrom(configurations.named("annotationProcessor").get())
}

configurations.named("componentTestCompileOnly").configure {
    extendsFrom(configurations.named("compileOnly").get())
}

configurations.named("componentTestImplementation").configure {
    extendsFrom(configurations.named("testImplementation").get())
}

tasks.register("assembleComponentTest") {
    dependsOn("compileComponentTestJava", "processComponentTestResources")
}

tasks.named("assemble") { dependsOn("assembleComponentTest") }

tasks.named("check") { dependsOn(testing.suites.named("componentTest")) }

tasks.named("componentTest") {
    finalizedBy(tasks.named("jacocoComponentTestReport"))
    extensions.configure<JacocoTaskExtension> {
        setDestinationFile(
            layout.buildDirectory.file("jacoco/jacoco-component-test.exec").get().asFile
        )
    }
}

tasks.register("jacocoComponentTestReport", JacocoReport::class) {
    sourceSets(sourceSets.main.get())
    dependsOn(tasks.named("componentTest"))
    executionData(layout.buildDirectory.file("jacoco/jacoco-component-test.exec").get().asFile)
    reports.xml.required = true
    reports.xml.outputLocation =
        layout.buildDirectory.file("jacoco/jacoco-component-test.xml").get().asFile
}
