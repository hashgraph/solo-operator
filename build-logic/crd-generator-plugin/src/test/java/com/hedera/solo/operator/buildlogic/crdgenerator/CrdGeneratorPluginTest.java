// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.buildlogic.crdgenerator;

import static org.junit.jupiter.api.Assertions.assertNotNull;

import org.gradle.api.Project;
import org.gradle.api.plugins.JavaPluginExtension;
import org.gradle.api.tasks.SourceSet;
import org.gradle.testfixtures.ProjectBuilder;
import org.junit.jupiter.api.Test;

/** A simple unit test for the 'com.hedera.solo.operator.buildlogic.crdGenerator' plugin. */
class CrdGeneratorPluginTest {
    @Test
    void pluginRegistersATask() {
        // Create a test project and apply the plugin
        Project project = ProjectBuilder.builder().build();
        project.getPlugins().apply("java");
        project.getPlugins().apply("com.hedera.solo.operator.buildlogic.crdGenerator");

        // Verify the result
        SourceSet sourceSet = project.getExtensions()
                .getByType(JavaPluginExtension.class)
                .getSourceSets()
                .getByName("main");
        assertNotNull(sourceSet);
        CrdGeneratorExtension crdGeneratorExtension = project.getExtensions()
                .create("crdGeneratorExtension", CrdGeneratorExtension.class, project, sourceSet);
        assertNotNull(crdGeneratorExtension);
        CrdGeneratorTask crdGeneratorTask = project.getTasks()
                .register("crdGenerator", CrdGeneratorTask.class, crdGeneratorExtension)
                .get();
        assertNotNull(crdGeneratorTask);
    }
}
