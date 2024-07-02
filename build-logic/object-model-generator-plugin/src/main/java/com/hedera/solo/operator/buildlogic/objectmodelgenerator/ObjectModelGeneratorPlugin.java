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

import java.io.File;
import java.nio.file.Path;
import java.util.function.BiFunction;
import java.util.function.Supplier;
import org.gradle.api.Plugin;
import org.gradle.api.Project;
import org.gradle.api.file.Directory;
import org.gradle.api.file.ProjectLayout;
import org.gradle.api.plugins.JavaPluginExtension;
import org.gradle.api.tasks.SourceSet;

/**
 * Object Model Generator plugin.
 */
public class ObjectModelGeneratorPlugin implements Plugin<Project> {
    @Override
    public void apply(Project project) {
        ObjectModelGeneratorExtension ext = project.getExtensions().create("objectModelGenerator", ObjectModelGeneratorExtension.class);
        apply(project, "objectModelGenerator", "compileJava", "main", ext.main, SourceSet.MAIN_SOURCE_SET_NAME);
        apply(project, "objectModelTestGenerator", "compileTestJava", "test", ext.test, SourceSet.TEST_SOURCE_SET_NAME);
    }

    void apply(final Project project, final String taskName, final String taskHook, final String dirName, final ObjectModelGeneratorFiles files, final String sourceSet) {
        ProjectLayout layout = project.getLayout();
        project.getTasks().register(taskName, ObjectModelGeneratorTask.class, task -> {
            task.sourceCrd = path(files.sourceCrd, () -> layout.getProjectDirectory().dir("src/" + dirName + "/k8s/crd"));
            task.targetCrd = path(files.targetCrd, () -> layout.getBuildDirectory().dir("generated/object-model-generator/" + dirName + "/crd").get());
            task.targetJava = path(files.targetJava, () -> layout.getBuildDirectory().dir("generated/object-model-generator/" + dirName + "/java").get());
            if (task.sourceCrd.toFile().exists()) {
                project.getExtensions().getByType(JavaPluginExtension.class).getSourceSets().findByName(sourceSet)
                        .java(s -> s.srcDir(task.targetJava));
            } else {
                task.onlyIf($ -> false);
            }
        });
        project.getTasks().getByName(taskHook).dependsOn(taskName);
    }

    Path path(final File file, final Supplier<Directory> supplier) {
        return (null != file ? file : supplier.get().getAsFile()).toPath();
    }
}
