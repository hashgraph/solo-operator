// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.buildlogic.gitextractor;

import static org.junit.jupiter.api.Assertions.*;

import org.gradle.api.Project;
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
        assertNotNull(project.getTasks().findByName("gitExtractor"));
    }
}
