// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.crdpostprocessor;

import static io.fabric8.crdv2.generator.CRDGenerator.getOutputName;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;
import com.fasterxml.jackson.dataformat.yaml.YAMLFactory;
import io.fabric8.crdv2.generator.CRDPostProcessor;
import io.fabric8.kubernetes.api.model.HasMetadata;
import io.fabric8.kubernetes.client.utils.ApiVersionUtil;
import io.fabric8.kubernetes.client.utils.KubernetesSerialization;
import java.io.File;
import java.io.IOException;
import java.nio.file.Path;
import java.util.Iterator;
import java.util.List;
import java.util.Map;
import java.util.stream.Collectors;
import java.util.stream.StreamSupport;

/**
 * This class implements the {@link CRDPostProcessor} interface to provide shared functionality for processing Custom
 * Resource Definitions (CRDs). Its main purpose is to restore missing specifications in the generated CRD files.
 */
public class SharedCrdPostProcessor implements CRDPostProcessor {
    ObjectMapper objectMapper = new ObjectMapper(new YAMLFactory());
    KubernetesSerialization kubernetesSerialization = new KubernetesSerialization();
    final Path restoreFromDir;

    /**
     * Constructs a new {@code SharedCrdPostProcessor} instance with the specified directory to restore from.
     *
     * @param restoreFromDir The path to the directory containing CRD files for processing.
     */
    public SharedCrdPostProcessor(Path restoreFromDir) {
        this.restoreFromDir = restoreFromDir;
    }

    @Override
    public HasMetadata process(HasMetadata crd, String crdSpecVersion) {
        String version = ApiVersionUtil.trimVersion(crd.getApiVersion());
        String crdName = crd.getMetadata().getName();
        String outputName = getOutputName(crdName, version);
        File source = null;
        for (String ext : List.of("yml", "yaml")) {
            source = restoreFromDir.resolve(outputName + "." + ext).toFile();
            if (source.exists()) {
                break;
            }
        }
        try {
            return kubernetesSerialization.unmarshal(
                    process(objectMapper.readTree(source), objectMapper.readTree(kubernetesSerialization.asYaml(crd))));
        } catch (IOException e) {
            throw new RuntimeException(e);
        }
    }

    /**
     * Processes the JSON representations of two CRD specifications to update and return the processed YAML content.
     *
     * @param in The input JSON node representing the source CRD specification.
     * @param out The output JSON node representing the target CRD specification.
     * @return A string representation of the processed YAML for the CRD
     * @throws JsonProcessingException If there's an issue with JSON processing.
     */
    protected String process(JsonNode in, JsonNode out) throws JsonProcessingException {
        update(map(in), map(out));
        return objectMapper.writeValueAsString(out);
    }

    /**
     * Maps the fields of a given JSON node to a key-value map for easier manipulation and updating.
     *
     * @param jsonNode The input JSON node to be mapped.
     * @return A map containing the fields of the JSON node as keys and their corresponding values
     */
    protected Map<String, JsonNode> map(JsonNode jsonNode) {
        return StreamSupport.stream(jsonNode.at("/spec/versions").spliterator(), false)
                .collect(Collectors.toMap(e -> e.get("name").asText(), e -> e.at("/schema/openAPIV3Schema")));
    }

    /**
     * Recursively updates the output map with values from the input map, ensuring all nodes are processed.
     *
     * @param in The input map of JSON nodes to be updated into the output map.
     * @param out The output map that is being updated by this method.
     */
    protected void update(Map<String, JsonNode> in, Map<String, JsonNode> out) {
        for (String key : out.keySet()) {
            if (in.containsKey(key)) {
                update(in.get(key), out.get(key));
            }
        }
    }

    /**
     * Recursively updates the nodes of the output JSON tree with values from the input JSON tree, handling both object
     * and leaf node updates.
     *
     * @param in The input JSON node to be updated into the output JSON node.
     * @param out The output JSON node that is being updated by this method.
     */
    protected void update(JsonNode in, JsonNode out) {
        for (Iterator<Map.Entry<String, JsonNode>> iterator = in.fields(); iterator.hasNext(); ) {
            Map.Entry<String, JsonNode> entry = iterator.next();
            if (entry.getValue() instanceof JsonNode inObjectNode) {
                if (!out.has(entry.getKey()) && out instanceof ObjectNode outObjectNode) {
                    outObjectNode.set(entry.getKey(), inObjectNode);
                    continue;
                }
            }
            update(entry.getValue(), out.get(entry.getKey()));
        }
    }
}
