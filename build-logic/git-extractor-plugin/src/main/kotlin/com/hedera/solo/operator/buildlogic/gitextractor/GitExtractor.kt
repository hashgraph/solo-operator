// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.buildlogic.gitextractor

import org.gradle.api.NamedDomainObjectProvider
import org.gradle.api.Project
import org.gradle.api.tasks.SourceSet
import org.gradle.kotlin.dsl.create
import org.gradle.kotlin.dsl.register

class GitExtractor(private val project: Project) {
    fun register(
        sourceSetProvider: NamedDomainObjectProvider<SourceSet>,
        initBlock: GitExtractorExtension.() -> Unit = {}
    ) {
        var sourceSet = sourceSetProvider.get()
        val taskName = sourceSet.getTaskName("generate", "git-extractor")
        var extension =
            project.extensions
                .create<GitExtractorExtension>(taskName + "Extension", sourceSet)
                .apply(initBlock)
        project.tasks.register<GitExtractorTask>(taskName, extension).get()
        sourceSet.resources.srcDir(extension.extractionDirectory)
        project.tasks.named(sourceSet.processResourcesTaskName).configure { dependsOn(taskName) }
    }
}
