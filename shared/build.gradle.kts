// SPDX-License-Identifier: Apache-2.0

plugins { `java-library` }

group = "com.hedera.solo.operator"

version = "1.0.0"

repositories {
    // Source Maven Central Artifacts from JFrog Artifactory
    maven { url = uri("https://artifacts.swirldslabs.io/artifactory/central-maven-external/") }
}

dependencies { api(libs.fabric8.crd.generator.api.v2) }
