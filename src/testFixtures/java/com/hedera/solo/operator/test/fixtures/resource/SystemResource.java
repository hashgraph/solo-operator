// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.fixtures.resource;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.util.Objects;

public class SystemResource {
    private SystemResource() {}

    public static String getSystemResourceAsString(final String name) {
        try {
            return new String(
                    Objects.requireNonNull(ClassLoader.getSystemResourceAsStream(name))
                            .readAllBytes(),
                    StandardCharsets.UTF_8);
        } catch (IOException e) {
            throw new RuntimeException(e);
        }
    }
}
