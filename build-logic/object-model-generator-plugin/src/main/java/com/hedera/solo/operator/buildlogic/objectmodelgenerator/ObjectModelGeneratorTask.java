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

    @Inject
    public ObjectModelGeneratorTask(ObjectModelGeneratorExtension extension) {
        this.extension = extension;
        this.sourceCrd = extension.sourceCrd.toPath();
        this.sourceOpenAPI = extension.sourceOpenAPI.toPath();
        this.targetCrd = extension.targetCrd.toPath();
        this.targetJava = extension.targetJava.toPath();
    }

    public Path getSourceCrd() {
        return sourceCrd;
    }

    public void setSourceCrd(Path sourceCrd) {
        this.sourceCrd = sourceCrd;
    }

    public Path getSourceOpenAPI() {
        return sourceOpenAPI;
    }

    public void setSourceOpenAPI(Path sourceOpenAPI) {
        this.sourceOpenAPI = sourceOpenAPI;
    }

    public Path getTargetCrd() {
        return targetCrd;
    }

    public void setTargetCrd(Path targetCrd) {
        this.targetCrd = targetCrd;
    }

    public Path getTargetJava() {
        return targetJava;
    }

    public void setTargetJava(Path targetJava) {
        this.targetJava = targetJava;
    }

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
