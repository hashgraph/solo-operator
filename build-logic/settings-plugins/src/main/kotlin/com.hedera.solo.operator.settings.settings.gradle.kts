// SPDX-License-Identifier: Apache-2.0

pluginManagement {
    repositories {
        gradlePluginPortal()
        maven { url = uri("https://artifacts.swirldslabs.io/artifactory/central-maven-external/") }
    }
}

plugins {
    id("com.gradle.enterprise")
    id("org.gradle.toolchains.foojay-resolver-convention")
}

val isCiServer = System.getenv().containsKey("CI")

// Enable Gradle Build Scan
gradleEnterprise {
    buildScan {
        termsOfServiceUrl = "https://gradle.com/terms-of-service"
        termsOfServiceAgree = "yes"

        if (isCiServer) {
            publishAlways()
            tag("CI")
        }
    }
}

val gradleCacheUsername: String? = System.getenv("GRADLE_CACHE_USERNAME")
val gradleCachePassword: String? = System.getenv("GRADLE_CACHE_PASSWORD")
val gradleCacheAuthorized =
    (gradleCacheUsername?.isNotEmpty() ?: false) && (gradleCachePassword?.isNotEmpty() ?: false)

buildCache {
    remote<HttpBuildCache> {
        url = uri("https://cache.gradle.hedera.svcs.eng.swirldslabs.io/cache/")
        isPush = isCiServer && gradleCacheAuthorized

        isUseExpectContinue = true
        isEnabled = !gradle.startParameter.isOffline

        if (isCiServer && gradleCacheAuthorized) {
            credentials {
                username = gradleCacheUsername
                password = gradleCachePassword
            }
        }
    }
}
