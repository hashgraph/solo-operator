// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.render;

import static com.hedera.solo.operator.library.generic.Nullable.orEmptyMap;

import java.util.Map;
import java.util.stream.Collectors;

public class RenderUtils {
    private RenderUtils() {}

    public static String join(final Map<String, String> map, final String delimiter) {
        return orEmptyMap(() -> map).entrySet().stream()
                .map(e -> e.getKey() + delimiter + e.getValue())
                .collect(Collectors.joining("\n", "", "\n"));
    }
}
