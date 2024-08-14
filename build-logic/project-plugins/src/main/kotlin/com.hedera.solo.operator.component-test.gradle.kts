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

tasks.named("assemble") { dependsOn(tasks.named("compileComponentTestJava")) }

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
