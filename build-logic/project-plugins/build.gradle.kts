// SPDX-License-Identifier: Apache-2.0

plugins {
    `kotlin-dsl`
    kotlin("jvm") version "2.1.0"
}

repositories { gradlePluginPortal() }

dependencies {
    implementation("com.diffplug.spotless:spotless-plugin-gradle:7.0.2")
    implementation("net.swiftzer.semver:semver:2.1.0")
}
