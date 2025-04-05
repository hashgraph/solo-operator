// SPDX-License-Identifier: Apache-2.0

pluginManagement {
    val quarkusPluginVersion: String by settings
    val quarkusPluginId: String by settings

    includeBuild("build-logic")

    plugins { id(quarkusPluginId) version quarkusPluginVersion }
}

plugins { id("com.hedera.solo.operator.settings") }

rootProject.name = "solo-operator"
