// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.buildlogic.crdgenerator

import org.gradle.api.NamedDomainObjectProvider
import org.gradle.api.Project
import org.gradle.api.tasks.SourceSet
import org.gradle.kotlin.dsl.create
import org.gradle.kotlin.dsl.register

class CrdGenerator(private val project: Project) {
    fun register(
        sourceSetProvider: NamedDomainObjectProvider<SourceSet>,
        initBlock: CrdGeneratorExtension.() -> Unit = {}
    ) {
        var sourceSet = sourceSetProvider.get()
        val taskName = sourceSet.getTaskName("generate", "crd")
        var extension =
            project.extensions
                .create<CrdGeneratorExtension>(taskName + "Extension", sourceSet)
                .apply(initBlock)
        project.tasks.register<CrdGeneratorTask>(taskName, extension).get()
        project.tasks.named(sourceSet.classesTaskName).configure { dependsOn(taskName) }
    }
}
