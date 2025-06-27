// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.fixtures.resource;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.util.Objects;

/** Utility class for retrieving system resources as strings. */
public class SystemResource {
    /** Private constructor to prevent instantiation of this utility class. */
    private SystemResource() {}

    /**
     * Retrieves the content of a system resource as a string.
     *
     * @param name The name of the resource to be retrieved.
     * @return The content of the specified resource as a String
     * @throws RuntimeException if the resource cannot be found or read.
     */
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
