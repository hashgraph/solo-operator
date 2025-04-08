// SPDX-License-Identifier: Apache-2.0

plugins { `kotlin-dsl` }

repositories { gradlePluginPortal() }

dependencies {
    implementation("com.diffplug.spotless:spotless-plugin-gradle:7.0.3")
    implementation("net.swiftzer.semver:semver:2.0.0")
}
