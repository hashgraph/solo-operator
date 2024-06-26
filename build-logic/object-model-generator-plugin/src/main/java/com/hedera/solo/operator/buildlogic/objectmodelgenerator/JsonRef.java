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
     * @param path path to the source file
     * @param root node to operate on
     * @throws IOException
     */
    static public void replace(ObjectMapper mapper, Path path, JsonNode root) throws IOException {
        new JsonRef(mapper).process(path, root);
    }

    private final ObjectMapper mapper;
    private final Map<Path, JsonNode> map = new HashMap<>();

    private JsonRef(ObjectMapper mapper) {
        this.mapper = mapper;
    }

    private JsonNode process(Path path) throws IOException {
        path = path.toAbsolutePath().normalize();
        JsonNode root = map.get(path);
        if (null != root) {
            return root;
        }
        return process(path, mapper.readTree(path.toFile()));
    }

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
