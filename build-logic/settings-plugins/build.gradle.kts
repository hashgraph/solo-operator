// SPDX-License-Identifier: Apache-2.0

plugins { `kotlin-dsl` }

repositories { gradlePluginPortal() }

dependencies {
    implementation("org.gradle.toolchains:foojay-resolver:0.7.0")
    implementation("com.gradle:gradle-enterprise-gradle-plugin:3.15.1")
}
