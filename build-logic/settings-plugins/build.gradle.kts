// SPDX-License-Identifier: Apache-2.0

plugins { `kotlin-dsl` }

repositories { gradlePluginPortal() }

dependencies {
    implementation(libs.foojay.resolver)
    implementation(libs.gradle.enterprise)
}
