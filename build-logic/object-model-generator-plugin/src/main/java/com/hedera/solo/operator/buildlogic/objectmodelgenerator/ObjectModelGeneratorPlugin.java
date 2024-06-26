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

import org.gradle.api.Plugin;
import org.gradle.api.Project;
import org.gradle.api.file.Directory;
import org.gradle.api.file.ProjectLayout;
import org.gradle.api.plugins.JavaPluginExtension;
import org.gradle.api.tasks.SourceSet;

import java.io.File;
import java.nio.file.Path;
import java.util.function.BiFunction;
import java.util.function.Supplier;

/**
 * Object Model Generator plugin.
 */
public class ObjectModelGeneratorPlugin implements Plugin<Project> {
    public void apply(Project project) {
        ObjectModelGeneratorExtension ext = project.getExtensions().create("objectModelGenerator", ObjectModelGeneratorExtension.class);
        project.getTasks().getByName("compileJava").dependsOn("objectModelGenerator");
        project.getTasks().register("objectModelGenerator", ObjectModelGeneratorTask.class, task -> {
            ProjectLayout layout = project.getLayout();
            BiFunction<File, Supplier<Directory>, Path> path = (file, supplier) -> (null != file ? file : supplier.get().getAsFile()).toPath();
            task.sourceCrd = path.apply(ext.sourceCrd, () -> layout.getProjectDirectory().dir("src/main/k8s/crd"));
            task.targetCrd = path.apply(ext.targetCrd, () -> layout.getBuildDirectory().dir("generated/objectmodelgenerator/crd").get());
            task.targetJava = path.apply(ext.targetJava, () -> layout.getBuildDirectory().dir("generated/objectmodelgenerator/sources/main/java").get());
            project.getExtensions().getByType(JavaPluginExtension.class).getSourceSets().findByName(SourceSet.MAIN_SOURCE_SET_NAME)
                    .java(s -> s.srcDir(task.targetJava));
        });
    }
}
