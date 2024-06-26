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

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.MappingIterator;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.SequenceWriter;
import com.fasterxml.jackson.databind.node.ObjectNode;
import com.fasterxml.jackson.dataformat.yaml.YAMLFactory;
import io.fabric8.java.generator.Config;
import io.fabric8.java.generator.FileJavaGenerator;
import org.gradle.api.DefaultTask;
import org.gradle.api.tasks.CacheableTask;
import org.gradle.api.tasks.InputDirectory;
import org.gradle.api.tasks.OutputDirectory;
import org.gradle.api.tasks.PathSensitive;
import org.gradle.api.tasks.PathSensitivity;
import org.gradle.api.tasks.TaskAction;

import java.io.File;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.stream.Stream;

@CacheableTask
public class ObjectModelGeneratorTask extends DefaultTask {
    YAMLFactory yamlFactory = new YAMLFactory();
    ObjectMapper mapper = new ObjectMapper(yamlFactory);
    @PathSensitive(PathSensitivity.ABSOLUTE)
    @InputDirectory
    Path sourceCrd;
    @OutputDirectory
    Path targetCrd;
    @OutputDirectory
    Path targetJava;

    public Path getSourceCrd() {
        return sourceCrd;
    }

    public Path getTargetCrd() {
        return targetCrd;
    }

    public Path getTargetJava() {
        return targetJava;
    }

    @TaskAction
    public void generateObjectModel() throws IOException {
        ObjectModelGeneratorExtension ext = getProject().getExtensions().getByType(ObjectModelGeneratorExtension.class);
        targetCrd.toFile().mkdirs();
        targetJava.toFile().mkdirs();
        try (Stream<Path> stream = Files.walk(sourceCrd)) {
            stream.filter(path -> path.toFile().isFile()).forEach(path -> process(ext, path));
        }
        for (File file : ext.removeFiles) {
            targetJava.resolve(getProject().getLayout().getProjectDirectory().getAsFile().toPath().relativize(file.toPath())).toFile().delete();
        }
    }

    void process(ObjectModelGeneratorExtension ext, Path file) {
        Config config = ext.configBuilderOverrides.getOrDefault(file.getFileName().toString(), ext.defaultConfigBuilder).build();
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
