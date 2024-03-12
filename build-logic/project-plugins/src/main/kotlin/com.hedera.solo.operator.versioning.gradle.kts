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

import Utils.Companion.updateStringInFile
import net.swiftzer.semver.SemVer

val productVersion = rootProject.version

tasks.register("showVersion") {
    group = "versioning"

    inputs.property("version", productVersion)

    doLast { println(inputs.properties["version"]) }
}

tasks.register("githubVersionSummary") {
    group = "github"

    inputs.property("version", productVersion)
    outputs.file(
        providers
            .environmentVariable("GITHUB_STEP_SUMMARY")
            .orElse(
                provider {
                    throw IllegalArgumentException(
                        "This task may only be run in a Github Actions CI environment! " +
                            "Unable to locate the GITHUB_STEP_SUMMARY environment variable."
                    )
                }
            )
    )

    doLast {
        Utils.generateProjectVersionReport(
            inputs.properties["version"] as String,
            outputs.files.singleFile.outputStream().buffered()
        )
    }
}

tasks.register("versionAsPrefixedCommit") {
    group = "versioning"

    @Suppress("UnstableApiUsage")
    inputs.property(
        "commit",
        providers
            .exec { commandLine("git", "rev-parse", "HEAD") }
            .standardOutput
            .asText
            .map { it.trim().substring(0, 7) }
    )
    inputs.property("commitPrefix", providers.gradleProperty("commitPrefix").orElse("adhoc"))
    inputs.property("version", productVersion)
    outputs.file(layout.projectDirectory.file("gradle.properties"))

    doLast {
        val newPrerel =
            inputs.properties["commitPrefix"].toString() +
                ".x" +
                inputs.properties["commit"].toString().take(8)
        val currVer = SemVer.parse(inputs.properties["version"] as String)
        val newVer = SemVer(currVer.major, currVer.minor, currVer.patch, newPrerel)
        updateStringInFile(outputs.files.singleFile, "version=", "version=${newVer}")
    }
}

tasks.register("versionAsSnapshot") {
    group = "versioning"

    inputs.property("version", productVersion)
    outputs.file(layout.projectDirectory.file("gradle.properties"))

    doLast {
        val currVer = SemVer.parse(inputs.properties["version"] as String)
        val newVer = SemVer(currVer.major, currVer.minor, currVer.patch, "SNAPSHOT")

        updateStringInFile(outputs.files.singleFile, "version=", "version=${newVer}")
    }
}

tasks.register("versionAsSpecified") {
    group = "versioning"

    inputs.property(
        "newVersion",
        providers
            .gradleProperty("newVersion")
            .orElse(
                provider {
                    throw IllegalArgumentException(
                        "No newVersion property provided! " +
                            "Please add the parameter -PnewVersion=<version> when running this task."
                    )
                }
            )
    )

    outputs.file(layout.projectDirectory.file("gradle.properties"))

    doLast {
        val newVer = SemVer.parse(inputs.properties["newVersion"] as String)
        updateStringInFile(outputs.files.singleFile, "version=", "version=${newVer}")
    }
}
