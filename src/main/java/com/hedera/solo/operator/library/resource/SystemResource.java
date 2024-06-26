package com.hedera.solo.operator.library.resource;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.util.Objects;

public class SystemResource {
    public static String getSystemResourceAsString(final String name) throws IOException {
        return new String(Objects.requireNonNull(ClassLoader.getSystemResourceAsStream(name)).readAllBytes(), StandardCharsets.UTF_8);
    }
}
