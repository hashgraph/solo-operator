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
package com.hedera.solo.operator.buildlogic.gitextractor;

import java.io.File;
import java.util.stream.Collectors;
import org.gradle.api.Project;
import org.gradle.api.Plugin;

/**
 * Git Extractor plugin.
 */
public class GitExtractorPluginPlugin implements Plugin<Project> {
    static final String name = "gitExtractor";
    public void apply(Project project) {
        GitExtractorExtension ext = project.getExtensions().create(name, GitExtractorExtension.class);
        project.getTasks().getByName("processResources").dependsOn(name);
        project.getTasks().getByName("spotlessKotlinGradle").dependsOn(name);
        project.getTasks().register(name, GitExtractorTask.class, task -> {
            task.outputDirectories = ext.extractions.stream().map(GitExtraction::destination).map(File::toPath).collect(Collectors.toSet());
        });
    }
}
