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

pluginManagement {
    val quarkusPluginVersion: String by settings
    val quarkusPluginId: String by settings

    includeBuild("build-logic")

    plugins { id(quarkusPluginId) version quarkusPluginVersion }
}

plugins { id("com.hedera.solo.operator.settings") }

rootProject.name = "solo-operator"
