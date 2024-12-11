// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.buildlogic.gitextractor;

import static org.junit.jupiter.api.Assertions.*;

import org.gradle.api.Project;
import org.gradle.api.plugins.JavaPluginExtension;
import org.gradle.api.tasks.SourceSet;
import org.gradle.testfixtures.ProjectBuilder;
import org.junit.jupiter.api.Test;

/** A simple unit test for the 'com.hedera.solo.operator.buildlogic.gitExtractor' plugin. */
class GitExtractorPluginTest {
    @Test
    void pluginRegistersATask() {
        // Create a test project and apply the plugin
        Project project = ProjectBuilder.builder().build();
        project.getPlugins().apply("java");
        project.getPlugins().apply("com.hedera.solo.operator.buildlogic.gitExtractor");

        // Verify the result
        SourceSet sourceSet = project.getExtensions()
                .getByType(JavaPluginExtension.class)
                .getSourceSets()
                .getByName("main");
        assertNotNull(sourceSet);
        GitExtractorExtension gitExtractorExtension = project.getExtensions()
                .create("gitExtractorExtension", GitExtractorExtension.class, project, sourceSet);
        assertNotNull(gitExtractorExtension);
        GitExtractorTask gitExtractorTask = project.getTasks()
                .register("gitExtractor", GitExtractorTask.class, gitExtractorExtension)
                .get();
        assertNotNull(gitExtractorTask);
    }
}
