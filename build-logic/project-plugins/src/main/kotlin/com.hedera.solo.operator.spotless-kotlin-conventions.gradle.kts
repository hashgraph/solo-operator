// SPDX-License-Identifier: Apache-2.0

plugins { id("com.diffplug.spotless") }

spotless {
    kotlinGradle {
        target("**/*.gradle.kts")
        targetExclude("**/build/**/*.gradle.kts")
        ktfmt().kotlinlangStyle()

        licenseHeader(
            "// SPDX-License-Identifier: Apache-2.0\n\n",
            "(import|plugins|pluginManagement|dependencyResolutionManagement|repositories|include|includeBuild|//spotless\\(ignore-empty\\)|// spotless\\(ignore-empty\\)|//\\s*pragma\\s+comment)"
        )
    }
}
