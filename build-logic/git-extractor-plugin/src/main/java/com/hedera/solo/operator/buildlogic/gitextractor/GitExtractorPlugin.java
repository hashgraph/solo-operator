// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.buildlogic.gitextractor;

import java.util.stream.Collectors;
import org.gradle.api.Plugin;
import org.gradle.api.Project;
import org.gradle.api.file.Directory;
import org.gradle.api.plugins.JavaPluginExtension;
import org.gradle.api.tasks.SourceSet;
import org.gradle.api.tasks.SourceSetContainer;
import org.jetbrains.annotations.NotNull;

/** Git Extractor plugin. */
public class GitExtractorPlugin implements Plugin<Project> {
    static final String name = "gitExtractor";

    public void apply(final @NotNull Project project) {
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
    }

    static Directory extractionDirectory(final Project project) {
        return project.getLayout()
                .getBuildDirectory()
                .dir("generated/git-extractor/src/main/resources")
                .get();
    }
}
