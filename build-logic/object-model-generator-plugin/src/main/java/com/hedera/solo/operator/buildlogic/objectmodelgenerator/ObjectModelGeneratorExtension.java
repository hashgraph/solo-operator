// SPDX-License-Identifier: Apache-2.0

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
