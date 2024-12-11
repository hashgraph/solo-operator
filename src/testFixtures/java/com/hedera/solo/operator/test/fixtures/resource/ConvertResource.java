// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.fixtures.resource;

import static com.hedera.solo.operator.test.fixtures.resource.SystemResource.getSystemResourceAsStringUnchecked;

import java.io.IOException;
import java.util.Map;
import java.util.Properties;
import java.util.stream.Collectors;

public class ConvertResource {
    private ConvertResource() {}

    public static Map<String, String> properties(final String name) {
        Properties properties = new Properties();
        try {
            properties.load(ClassLoader.getSystemResourceAsStream(name));
        } catch (IOException e) {
            throw new RuntimeException(e);
        }
        return properties.entrySet().stream()
                .collect(Collectors.toMap(
                        e -> e.getKey().toString(), e -> e.getValue().toString()));
    }

    public static Map<String, String> settings(final String name) {
        return getSystemResourceAsStringUnchecked(name)
                .lines()
                .map(line -> line.replaceFirst("\\s*#.*", ""))
                .filter(line -> !line.isEmpty())
                .map(ConvertResource::entry)
                .collect(Collectors.toMap(Map.Entry::getKey, Map.Entry::getValue));
    }

    private static Map.Entry<String, String> entry(final String line) {
        int index = line.indexOf(',');
        return index == -1 ? Map.entry(line, "") : Map.entry(line.substring(0, index), line.substring(index + 1));
    }
}
