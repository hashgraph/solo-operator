// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.buildlogic.objectmodelgenerator;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.MappingIterator;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.SequenceWriter;
import com.fasterxml.jackson.databind.node.ObjectNode;
import com.fasterxml.jackson.dataformat.yaml.YAMLFactory;
import io.fabric8.java.generator.Config;
import io.fabric8.java.generator.FileJavaGenerator;
import java.io.File;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.stream.Stream;
import javax.inject.Inject;
import org.gradle.api.DefaultTask;
import org.gradle.api.tasks.CacheableTask;
import org.gradle.api.tasks.InputDirectory;
import org.gradle.api.tasks.OutputDirectory;
import org.gradle.api.tasks.PathSensitive;
import org.gradle.api.tasks.PathSensitivity;
import org.gradle.api.tasks.TaskAction;

/**
 * The ObjectModelGeneratorTask class is a Gradle task that generates object model from YAML CRD (Custom Resource Definition) files and OpenAPI specifications.
 */
@CacheableTask
public class ObjectModelGeneratorTask extends DefaultTask {
    YAMLFactory yamlFactory = new YAMLFactory();
    ObjectMapper mapper = new ObjectMapper(yamlFactory);
    ObjectModelGeneratorExtension extension;
    @PathSensitive(PathSensitivity.ABSOLUTE)
    @InputDirectory
    Path sourceCrd;
    @PathSensitive(PathSensitivity.ABSOLUTE)
    @InputDirectory
    Path sourceOpenAPI;
    @OutputDirectory
    Path targetCrd;
    @OutputDirectory
    Path targetJava;

    /**
     * Constructs a new ObjectModelGeneratorTask with the given extension.
     *
     * @param extension The extension containing configuration for the task.
     */
    @Inject
    public ObjectModelGeneratorTask(ObjectModelGeneratorExtension extension) {
        this.extension = extension;
        this.sourceCrd = extension.sourceCrd.toPath();
        this.sourceOpenAPI = extension.sourceOpenAPI.toPath();
        this.targetCrd = extension.targetCrd.toPath();
        this.targetJava = extension.targetJava.toPath();
    }

    /**
     * Gets the source CRD directory path.
     *
     * @return The path to the source CRD directory
     */
    public Path getSourceCrd() {
        return sourceCrd;
    }

    /**
     * Sets the source CRD directory path.
     *
     * @param sourceCrd The new path for the source CRD directory.
     */
    public void setSourceCrd(Path sourceCrd) {
        this.sourceCrd = sourceCrd;
    }

    /**
     * Gets the source OpenAPI directory path.
     *
     * @return The path to the source OpenAPI directory
     */
    public Path getSourceOpenAPI() {
        return sourceOpenAPI;
    }

    /**
     * Sets the source OpenAPI directory path.
     *
     * @param sourceOpenAPI The new path for the source OpenAPI directory.
     */
    public void setSourceOpenAPI(Path sourceOpenAPI) {
        this.sourceOpenAPI = sourceOpenAPI;
    }

    /**
     * Gets the target CRD directory path.
     *
     * @return The path to the target CRD directory
     */
    public Path getTargetCrd() {
        return targetCrd;
    }

    /**
     * Sets the target CRD directory path.
     *
     * @param targetCrd The new path for the target CRD directory.
     */
    public void setTargetCrd(Path targetCrd) {
        this.targetCrd = targetCrd;
    }

    /**
     * Gets the target Java directory path.
     *
     * @return The path to the target Java directory
     */
    public Path getTargetJava() {
        return targetJava;
    }

    /**
     * Sets the target Java directory path.
     *
     * @param targetJava The new path for the target Java directory.
     */
    public void setTargetJava(Path targetJava) {
        this.targetJava = targetJava;
    }

    /**
     * Generates the object model by processing CRD files and OpenAPI specifications.
     *
     * @throws IOException If an I/O error occurs.
     */
    @TaskAction
    public void generateObjectModel() throws IOException {
        targetCrd.toFile().mkdirs();
        targetJava.toFile().mkdirs();
        try (Stream<Path> stream = Files.walk(sourceCrd)) {
            stream.filter(path -> path.toFile().isFile()).forEach(this::process);
        }
        for (File file : extension.removeFiles) {
            targetJava.resolve(getProject().getLayout().getProjectDirectory().getAsFile().toPath().relativize(file.toPath())).toFile().delete();
        }
    }

    /**
     * Processes a CRD file to generate the corresponding Java model files.
     *
     * @param file The path to the CRD file.
     * @throws RuntimeException If an error occurs during processing.
     */
    void process(Path file) {
        Config config = extension.configBuilderOverrides.getOrDefault(file.getFileName().toString(), extension.defaultConfigBuilder).build();
        if (config.getFilesSuffixes().stream().noneMatch(suffix -> file.getFileName().toString().endsWith(suffix))) {
            return;
        }
        File output = targetCrd.resolve(sourceCrd.relativize(file)).toFile();
        try (SequenceWriter sw = mapper.writerWithDefaultPrettyPrinter().writeValues(output)) {
            for (MappingIterator<JsonNode> it = mapper.readValues(yamlFactory.createParser(file.toFile()), JsonNode.class); it.hasNext(); ) {
                JsonNode node = it.next();
                JsonRef.replace(mapper, file, node);
                if (node instanceof ObjectNode && "CustomResourceDefinition".equals(node.get("kind").asText())) {
                    JavaTypes.update(config, node);
                    node.findParents("x-java-type").stream().filter($ -> $ instanceof ObjectNode)
                            .forEach($ -> ((ObjectNode) $).remove("x-java-type"));
                }
                sw.write(node);
            }
        } catch (IOException exception) {
            throw new RuntimeException(exception);
        }
        FileJavaGenerator fileJavaGenerator = new FileJavaGenerator(config, output);
        fileJavaGenerator.run(targetJava.toFile());
    }
}
