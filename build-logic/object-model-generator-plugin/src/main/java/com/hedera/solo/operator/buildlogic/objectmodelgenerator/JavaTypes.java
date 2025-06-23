// SPDX-License-Identifier: Apache-2.0

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
 * Utility class to generate classname remapping for CRD annotated with x-java-type.
 */
public class JavaTypes {
    /**
     * Update config with classname mappings from generated classname to x-java-type classname.
     *
     * @param config any overrides configured for the generated package names
     * @param crd    the CRD
     */
    public static void update(Config config, JsonNode crd) {
        config.getExistingJavaTypes().putAll(new JavaTypes(config.getPackageOverrides()).process(crd));
    }

    private final Deque<String> path = new ArrayDeque<>();
    private final Map<String, String> config = new HashMap<>();
    private final Map<String, String> packageOverrides;

    /**
     * Constructor for JavaTypes that initializes the package overrides.
     *
     * @param packageOverrides a map of package name overrides
     */
    private JavaTypes(Map<String, String> packageOverrides) {
        this.packageOverrides = packageOverrides;
    }

    /**
     * Processes the CRD to extract and update classname mappings.
     *
     * @param node the JsonNode representing the CRD
     * @return a map of classname remappings
     */
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

    /**
     * Retrieves a JsonNode by its name from the given node.
     *
     * @param node the root JsonNode to search within
     * @param name the name of the JsonNode to retrieve
     * @return an Optional containing the JsonNode if found, or empty if not
     */
    private Optional<JsonNode> get(JsonNode node, String name) {
        return Optional.ofNullable(node.get(name));
    }

    /**
     * Recursively processes the properties of a JSON schema to extract and update classname mappings.
     *
     * @param node the JsonNode representing the schema properties
     * @param name the base name for the current path in the schema hierarchy
     */
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
