// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.fixtures.resource;

import static com.hedera.solo.operator.test.fixtures.resource.SystemResource.getSystemResourceAsString;

import java.util.LinkedHashMap;
import java.util.Map;
import java.util.SequencedMap;
import java.util.regex.Pattern;

/** Provides utility methods for converting resource files into maps. */
public class ResourceConverter {
    private static final Pattern skip = Pattern.compile("\\s*(#.*)?");

    /** Private constructor to prevent instantiation of this utility class. */
    private ResourceConverter() {}

    /**
     * Converts a resource file into a map using the specified delimiter.
     *
     * @param name The name of the resource file to be converted.
     * @param delimiter The character used to separate keys and values in the resource file.
     * @return A LinkedHashMap containing the key-value pairs from the resource file
     */
    public static Map<String, String> asMap(final String name, final int delimiter) {
        SequencedMap<String, String> map = new LinkedHashMap<>();
        getSystemResourceAsString(name).lines().forEachOrdered(line -> put(map, delimiter, line));
        return map;
    }

    /**
     * Helper method to add key-value pairs to the map.
     *
     * @param map The LinkedHashMap where key-value pairs will be stored.
     * @param delimiter The character used to separate keys and values in the resource file.
     * @param line The line from the resource file that contains a potential key-value pair.
     * @throws IllegalStateException if a duplicate key is found in the map.
     */
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
