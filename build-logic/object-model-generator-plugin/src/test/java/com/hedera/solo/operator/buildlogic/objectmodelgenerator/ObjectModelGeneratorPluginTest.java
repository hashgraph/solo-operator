// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.buildlogic.objectmodelgenerator;

import static org.junit.jupiter.api.Assertions.assertNotNull;

import org.gradle.api.Project;
import org.gradle.api.plugins.JavaPluginExtension;
import org.gradle.api.tasks.SourceSet;
import org.gradle.testfixtures.ProjectBuilder;
import org.junit.jupiter.api.Test;

/**
 * A simple unit test for the 'com.hedera.solo.operator.buildlogic.objectModelGenerator' plugin.
 */
class ObjectModelGeneratorPluginTest {
    @Test
    void pluginRegistersATask() {
        // Create a test project and apply the plugin
        Project project = ProjectBuilder.builder().build();
        project.getPlugins().apply("java");
        project.getPlugins().apply("com.hedera.solo.operator.buildlogic.objectModelGenerator");

        // Verify the result
        SourceSet sourceSet = project.getExtensions().getByType(JavaPluginExtension.class).getSourceSets().getByName("main");
        assertNotNull(sourceSet);
        ObjectModelGeneratorExtension objectModelGeneratorExtension = project.getExtensions().create("objectModelGeneratorExtension", ObjectModelGeneratorExtension.class, project, sourceSet);
        assertNotNull(objectModelGeneratorExtension);
        ObjectModelGeneratorTask objectModelGeneratorTask = project.getTasks().register("objectModelGenerator", ObjectModelGeneratorTask.class, objectModelGeneratorExtension).get();
        assertNotNull(objectModelGeneratorTask);
    }
}
