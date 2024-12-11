package com.hedera.solo.operator.buildlogic.gitextractor

import org.gradle.api.NamedDomainObjectProvider
import org.gradle.api.Project
import org.gradle.api.tasks.SourceSet
import org.gradle.kotlin.dsl.create
import org.gradle.kotlin.dsl.register
import java.util.stream.Collectors

class GitExtractor(private val project: Project) {
    fun register(
        sourceSetProvider: NamedDomainObjectProvider<SourceSet>,
        initBlock: GitExtractorExtension.() -> Unit = {}
    ) {
        /*
        project.getTasks().getByName("processResources").dependsOn(name);
        GitExtractorExtension ext = project.getExtensions().create(name, GitExtractorExtension.class);
        SourceSetContainer sourceSets =
                project.getExtensions().getByType(JavaPluginExtension.class).getSourceSets();
        SourceSet mainSourceSet = sourceSets.getByName(SourceSet.MAIN_SOURCE_SET_NAME);
        project.getTasks().register(name, GitExtractorTask.class, task -> {
            Directory directory = extractionDirectory(project);
            task.outputDirectories = ext.extractions.stream()
                    .map(e -> directory.dir(e.destination()).getAsFile().toPath())
                    .collect(Collectors.toSet());
            mainSourceSet.getResources().srcDir(directory);
        });

    static Directory extractionDirectory(final Project project) {
        return project.getLayout()
                .getBuildDirectory()
                .dir("generated/git-extractor/src/main/resources")
                .get();
    }
        * */
        var sourceSet = sourceSetProvider.get()
        val taskName = sourceSet.getTaskName("generate", "git-extractor")
        var extension = project.extensions.create<GitExtractorExtension>(taskName + "Extension", sourceSet)
            .apply(initBlock)
        var task = project.tasks.register<GitExtractorTask>(taskName).get()
        var directory = project.layout.buildDirectory.dir("generated/git-extractor/src/" + sourceSet.name + "/resources").get()
        task.outputDirectories = extension.extractions.stream()
            .map { e -> directory.dir(e.destination).asFile.toPath() }
            .collect(Collectors.toSet())
        sourceSet.resources.srcDir(directory)
        project.tasks.named(sourceSet.processResourcesTaskName).configure {
            dependsOn(taskName)
        }
    }
}
