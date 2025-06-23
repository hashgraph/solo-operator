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

public class SharedCrdPostProcessor implements CRDPostProcessor {
    ObjectMapper objectMapper = new ObjectMapper(new YAMLFactory());
    KubernetesSerialization kubernetesSerialization = new KubernetesSerialization();
    final Path restoreFromDir;

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

    protected String process(JsonNode in, JsonNode out) throws JsonProcessingException {
        update(map(in), map(out));
        return objectMapper.writeValueAsString(out);
    }

    protected Map<String, JsonNode> map(JsonNode jsonNode) {
        return StreamSupport.stream(jsonNode.at("/spec/versions").spliterator(), false)
                .collect(Collectors.toMap(e -> e.get("name").asText(), e -> e.at("/schema/openAPIV3Schema")));
    }

    protected void update(Map<String, JsonNode> in, Map<String, JsonNode> out) {
        for (String key : out.keySet()) {
            if (in.containsKey(key)) {
                update(in.get(key), out.get(key));
            }
        }
    }

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
