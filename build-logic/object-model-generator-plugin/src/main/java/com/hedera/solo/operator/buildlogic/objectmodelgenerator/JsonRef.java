// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.buildlogic.objectmodelgenerator;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;
import java.io.IOException;
import java.nio.file.Path;
import java.util.HashMap;
import java.util.Map;

/**
 * Utility class to expand {@code $ref} fields.
 */
public class JsonRef {
    /**
     * Replace objects containing {@code $ref} with the object pointed to by {@code $ref}.
     *
     * @param mapper The ObjectMapper instance used for JSON parsing and manipulation.
     * @param path   Path to the source file containing the JSON data.
     * @param root   The root node of the JSON tree that needs its {@code $ref} fields expanded.
     * @throws IOException If there is an error reading or processing the JSON file.
     */
    static public void replace(ObjectMapper mapper, Path path, JsonNode root) throws IOException {
        new JsonRef(mapper).process(path, root);
    }

    private final ObjectMapper mapper;
    private final Map<Path, JsonNode> map = new HashMap<>();

    /**
     * Constructs a new JsonRef instance with the given ObjectMapper.
     *
     * @param mapper The ObjectMapper instance used for JSON parsing and manipulation.
     */
    private JsonRef(ObjectMapper mapper) {
        this.mapper = mapper;
    }

    /**
     * Processes the JSON file at the specified path to expand {@code $ref} fields.
     *
     * @param path Path to the source file containing the JSON data.
     * @return The root node of the processed JSON tree
     * @throws IOException If there is an error reading or processing the JSON file.
     */
    private JsonNode process(Path path) throws IOException {
        path = path.toAbsolutePath().normalize();
        JsonNode root = map.get(path);
        if (null != root) {
            return root;
        }
        return process(path, mapper.readTree(path.toFile()));
    }

    /**
     * Processes the JSON node tree to expand {@code $ref} fields.
     *
     * @param path The path to the source file containing the JSON data (used for resolving relative paths).
     * @param root The root node of the JSON tree that needs its {@code $ref} fields expanded.
     * @return The processed JSON node tree with expanded {@code $ref} fields
     * @throws IOException If there is an error reading or processing the JSON data.
     */
    private JsonNode process(Path path, JsonNode root) throws IOException {
        map.put(path, root);
        for (JsonNode node : root.findParents("$ref")) {
            if (node instanceof ObjectNode object) {
                String[] fields = (node.get("$ref").asText() + "##_").split("#");
                JsonNode refRoot = fields[0].isEmpty() ? root : process(path.resolveSibling(fields[0]));
                JsonNode refNode = fields[1].isEmpty() ? refRoot : refRoot.at(fields[1]);
                if (refNode instanceof ObjectNode refObject) {
                    object.removeAll();
                    object.setAll(refObject);
                }
            }
        }
        return root;
    }
}
