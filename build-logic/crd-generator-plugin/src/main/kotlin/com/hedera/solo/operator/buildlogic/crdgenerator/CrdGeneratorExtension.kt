// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.buildlogic.crdgenerator

import io.fabric8.crdv2.generator.CRDPostProcessor
import org.gradle.api.Project
import org.gradle.api.file.FileCollection
import org.gradle.api.tasks.SourceSet
import java.io.File

open class CrdGeneratorExtension(val project: Project, val sourceSet: SourceSet) {
    var classPathElements: FileCollection = project.files(sourceSet.java.classesDirectory)
    var filesToScan: FileCollection = classPathElements
    var outputDir: File = project.layout.buildDirectory.dir("generated/crd-generator/${sourceSet.name}").get().asFile
    var parentClassLoader: ClassLoader = Thread.currentThread().getContextClassLoader()
    var postProcessorClassName: String? = null
    var postProcessorClassPath: FileCollection = project.files()
    var postProcessorNewInstance: ((Class<out CRDPostProcessor>) -> CRDPostProcessor) =
        { clazz -> clazz.getConstructor().newInstance() }
}
