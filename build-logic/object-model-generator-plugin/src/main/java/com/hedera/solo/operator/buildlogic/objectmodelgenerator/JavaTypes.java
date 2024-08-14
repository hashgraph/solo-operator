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

import static io.fabric8.java.generator.nodes.AbstractJSONSchema2Pojo.packageName;
import static io.fabric8.java.generator.nodes.AbstractJSONSchema2Pojo.sanitizeString;

import com.fasterxml.jackson.databind.JsonNode;
import io.fabric8.java.generator.Config;
import java.util.ArrayDeque;
import java.util.Arrays;
import java.util.Deque;
import java.util.HashMap;
import java.util.Map;
import java.util.Optional;

/**
 * Utility class to generate classname remapping for CRD annotated with x-java-type
 */
public class JavaTypes {
    /**
     * Update config with classname mappings from generated classname to x-java-type classname.
     *
     * @param config any overrides configured for the generated package names
     * @param crd    the CRD
     * @return
     */
    public static void update(Config config, JsonNode crd) {
        config.getExistingJavaTypes().putAll(new JavaTypes(config.getPackageOverrides()).process(crd));
    }

    private final Deque<String> path = new ArrayDeque<>();
    private final Map<String, String> config = new HashMap<>();
    private final Map<String, String> packageOverrides;

    private JavaTypes(Map<String, String> packageOverrides) {
        this.packageOverrides = packageOverrides;
    }

    private Map<String, String> process(JsonNode node) {
        String basePackage = Arrays.stream(node.at("/spec/group").asText().replace('-', '_').split("\\.")).reduce((a, b) -> b + "." + a).orElse("");
        String kind = node.at("/spec/names/kind").asText();
        node.at("/spec/versions").forEach(version -> {
            String pkg = basePackage + (basePackage.isEmpty() ? "" : ".") + version.get("name").asText();
            path.addLast(packageOverrides.getOrDefault(pkg, pkg));
            JsonNode properties = version.at("/schema/openAPIV3Schema/properties");
            process(properties.get("spec"), kind + "Spec");
            process(properties.get("status"), kind + "Status");
            path.removeLast();
        });
        return config;
    }

    private Optional<JsonNode> get(JsonNode node, String name) {
        return Optional.ofNullable(node.get(name));
    }

    private void process(JsonNode node, String name) {
        if (null != node) {
            String type = get(node, "type").map(JsonNode::asText).orElse(null);
            JsonNode next = "array".equals(type) ? node.get("items") : node.get("additionalProperties");
            if (null != next) {
                process(next, name);
            } else if ("object".equals(type)) {
                String className = sanitizeString(Character.toUpperCase(name.charAt(0)) + name.substring(1));
                get(node, "x-java-type").map(JsonNode::asText).ifPresent(x -> config.put(String.join(".", path) + "." + className, x));
                get(node, "properties").map(JsonNode::fields).ifPresent(it -> it.forEachRemaining(e -> {
                    path.addLast(packageName(className));
                    process(e.getValue(), e.getKey());
                    path.removeLast();
                }));
            }
        }
    }
}
