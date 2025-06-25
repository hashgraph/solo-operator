// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.render;

import static com.hedera.solo.operator.library.generic.Nullable.orEmptyMap;

import java.util.Map;
import java.util.stream.Collectors;

/** Utility class for rendering maps into a string format with a specified delimiter. */
public class RenderUtils {
    /** Private constructor to prevent instantiation of this utility class. */
    private RenderUtils() {}

    /**
     * Joins the entries of a map into a single string, where each entry is separated by the given delimiter. The format
     * of each entry in the output string will be "key + delimiter + value".
     *
     * @param map the map to join
     * @param delimiter the delimiter to use between key-value pairs
     * @return a string representation of the map entries, separated by the specified delimiter and ending with a
     *     newline character
     */
    public static String join(final Map<String, String> map, final String delimiter) {
        return orEmptyMap(() -> map).entrySet().stream()
                .map(e -> e.getKey() + delimiter + e.getValue())
                .collect(Collectors.joining("\n", "", "\n"));
    }
}
