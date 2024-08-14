/*
 * Copyright (C) 2024 Hedera Hashgraph, LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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
