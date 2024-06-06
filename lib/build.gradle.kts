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
    `java-library`
    id("com.hedera.solo.operator.objectModelGenerator")
}

repositories {
    mavenLocal()
    maven {
        url = uri("https://oss.sonatype.org/content/repositories/snapshots/")
    }

    maven {
        url = uri("https://repo.maven.apache.org/maven2/")
    }
}

dependencies {
    annotationProcessor(libs.io.sundr.builder.annotations)
    annotationProcessor(libs.io.fabric8.kubernetes.client)
    annotationProcessor(libs.io.fabric8.crd.generator.apt)
//    annotationProcessor("org.slf4j:slf4j-simple:2.0.13")
//    testAnnotationProcessor(libs.io.fabric8.crd.generator.apt)
//    testAnnotationProcessor("org.slf4j:slf4j-nop:2.0.13")
    api(libs.io.javaoperatorsdk.operator.framework)
    api(libs.org.apache.logging.log4j.log4j.slf4j.impl)
    api(libs.org.takes.takes)
    api(libs.org.awaitility.awaitility)
    testImplementation(libs.io.javaoperatorsdk.operator.framework.junit.v5)
    compileOnly(libs.io.fabric8.crd.generator.apt)

    compileOnly(libs.io.sundr.builder.annotations)
    compileOnly(libs.lombok)

//    implementation("org.slf4j:slf4j-simple:2.0.13")
}

group = "io.javaoperatorsdk"
version = "4.8.4-SNAPSHOT"
description = "Operator SDK - Samples - WebPage"
java.sourceCompatibility = JavaVersion.VERSION_21

tasks.withType<JavaCompile>() {
    options.encoding = "UTF-8"
}

tasks.withType<Javadoc>() {
    options.encoding = "UTF-8"
}

tasks.named<Test>("test") {
    // Use JUnit Jupiter for unit tests.
    useJUnitPlatform()
}

objectModelGenerator {
    defaultConfigBuilder.uppercaseEnums(false)
    defaultConfigBuilder.objectExtraAnnotations(true)
    defaultConfigBuilder.generatedAnnotations(false)
}
