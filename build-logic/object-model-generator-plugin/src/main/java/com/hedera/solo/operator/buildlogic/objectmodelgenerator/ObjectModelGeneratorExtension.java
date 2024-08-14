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

import io.fabric8.java.generator.Config;
import java.io.File;
import java.util.HashMap;
import java.util.HashSet;
import java.util.Map;
import java.util.Set;
import org.gradle.api.Project;
import org.gradle.api.file.Directory;
import org.gradle.api.file.ProjectLayout;
import org.gradle.api.tasks.SourceSet;

public class ObjectModelGeneratorExtension {
    public ObjectModelGeneratorExtension(Project project, SourceSet sourceSet) {
        ProjectLayout layout = project.getLayout();
        String dirName = sourceSet.getName();
        Directory source = layout.getProjectDirectory().dir("src").dir(dirName).dir("k8s");
        sourceCrd = source.dir("crd").getAsFile();
        sourceOpenAPI = source.dir("openapi").getAsFile();
        Directory target = layout.getBuildDirectory().get().dir("generated/object-model-generator").dir(dirName);
        targetCrd = target.dir("crd").getAsFile();
        targetJava = target.dir("java").getAsFile();
    }

    /**
     * Builder for the default configuration
     */
    public Config.ConfigBuilder defaultConfigBuilder = Config.builder()
            .uppercaseEnums(false)
            .objectExtraAnnotations(true)
            .generatedAnnotations(false);
    /**
     * Source directory for CRDs
     */
    public File sourceCrd;
    /**
     * Source directory for OpenAPI
     */
    public File sourceOpenAPI;
    /**
     * Target directory for generated CRDs
     */
    public File targetCrd;
    /**
     * Target directory for generated Java source
     */
    public File targetJava;
    /**
     * Builder override for specific file named by map key
     */
    public Map<String, Config.ConfigBuilder> configBuilderOverrides = new HashMap<>();
    /**
     * Optional set of generated files to remove.
     */
    public Set<File> removeFiles = new HashSet<>();
}
