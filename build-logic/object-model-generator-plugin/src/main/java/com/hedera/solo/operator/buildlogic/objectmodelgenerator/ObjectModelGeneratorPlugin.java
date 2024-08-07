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
        ProjectLayout layout = project.getLayout();
        for (SourceSet sourceSet : project.getExtensions().getByType(JavaPluginExtension.class).getSourceSets()) {
            String dirName = sourceSet.getName();
            File sourceCrd = layout.getProjectDirectory().dir("src").dir(dirName).dir("k8s").dir("crd").getAsFile();
            if (sourceCrd.exists()) {
                String taskName = sourceSet.getTaskName("generate", "object-model");
                project.getTasks().register(taskName, ObjectModelGeneratorTask.class, task -> {
                    Directory target = layout.getBuildDirectory().get().dir("generated").dir("object-model-generator");
                    task.sourceCrd = sourceCrd.toPath();
                    task.targetCrd = target.dir(dirName).dir("crd").getAsFile().toPath();
                    task.targetJava = target.dir(dirName).dir("java").getAsFile().toPath();
                    sourceSet.java(s -> s.srcDir(task.targetJava));
                });
                project.getTasks().getByName(sourceSet.getCompileJavaTaskName()).dependsOn(taskName);

            }
        }
    }
}
