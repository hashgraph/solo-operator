package com.hedera.solo.operator.buildlogic.objectmodelgenerator

import org.gradle.api.NamedDomainObjectProvider
import org.gradle.api.Project
import org.gradle.api.tasks.SourceSet
import org.gradle.kotlin.dsl.create
import org.gradle.kotlin.dsl.register

class ObjectModelGenerator(private val project: Project) {
    fun register(
        sourceSetProvider: NamedDomainObjectProvider<SourceSet>,
        initBlock: ObjectModelGeneratorExtension.() -> Unit = {}
    ) {
        var sourceSet = sourceSetProvider.get()
        val taskName = sourceSet.getTaskName("generate", "object-model")
        var extension = project.extensions.create<ObjectModelGeneratorExtension>(taskName + "Extension", sourceSet)
            .apply({
                defaultConfigBuilder.packageOverrides(
                    mapOf("com.hedera.solo.operator.v1alpha1" to "com.hedera.solo.operator.model.v1alpha1")
                )
            })
            .apply(initBlock)
        var task = project.tasks.register<ObjectModelGeneratorTask>(taskName, extension).get()
        sourceSet.java.srcDir(task.targetJava)
        project.tasks.named(sourceSet.compileJavaTaskName).configure {
            dependsOn(taskName)
        }
    }
}