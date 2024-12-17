// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.fixtures.resource;

import static com.hedera.solo.operator.test.fixtures.resource.SystemResource.getSystemResourceAsString;

import java.util.LinkedHashMap;
import java.util.Map;
import java.util.regex.Pattern;

public class ResourceConverter {
    private static final Pattern skip = Pattern.compile("\\s*(#.*)?");

    private ResourceConverter() {}

    public static Map<String, String> asMap(final String name, final int delimiter) {
        LinkedHashMap<String, String> map = new LinkedHashMap<>();
        getSystemResourceAsString(name).lines().forEachOrdered(line -> put(map, delimiter, line));
        return map;
    }

    private static void put(final Map<String, String> map, final int delimiter, final String line) {
        if (skip.matcher(line).matches()) {
            return;
        }
        String key = line, value = "";
        int index = line.indexOf(delimiter);
        if (index != -1) {
            key = line.substring(0, index);
            value = line.substring(index + 1);
        }
        if (map.containsKey(key)) {
            throw new IllegalStateException("Duplicate key " + key);
        }
        map.put(key, value);
    }
}
