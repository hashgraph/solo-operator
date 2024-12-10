// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.container.oci;

import static com.fasterxml.jackson.databind.DeserializationFeature.FAIL_ON_UNKNOWN_PROPERTIES;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.ObjectReader;
import java.io.IOException;
import java.util.Arrays;
import java.util.HashMap;
import java.util.Map;
import java.util.Optional;
import java.util.function.Function;

/** Common base and factory for responses to OCI manifest requests. */
public abstract sealed class OCIManifest permits OCIImageIndex, OCIImageManifest {
    private static final Map<String, Function<byte[], ? extends OCIManifest>> constructors = constructors();
    private static final ObjectReader objectReader =
            new ObjectMapper().disable(FAIL_ON_UNKNOWN_PROPERTIES).reader();

    public static OCIManifest from(final String mediaType, final byte[] bytes) {
        return Optional.ofNullable(OCIManifest.constructors.get(mediaType))
                .map(ctor -> ctor.apply(bytes))
                .orElse(null);
    }

    public static String[] mediaTypes() {
        return constructors.keySet().toArray(String[]::new);
    }

    private static <T extends OCIManifest> Function<byte[], T> to(final Class<T> aClass) {
        return bytes -> {
            try {
                return objectReader.readValue(bytes, aClass);
            } catch (IOException e) {
                throw new RuntimeException(e);
            }
        };
    }

    private static <T extends OCIManifest> void add(
            final Map<String, Function<byte[], ? extends OCIManifest>> map,
            final Class<T> aClass,
            final String... mediaTypes) {
        Arrays.stream(mediaTypes).forEach(mediaType -> map.put(mediaType, to(aClass)));
    }

    private static Map<String, Function<byte[], ? extends OCIManifest>> constructors() {
        Map<String, Function<byte[], ? extends OCIManifest>> map = new HashMap<>();
        add(map, OCIImageIndex.class, OCIImageIndex.mediaTypes());
        add(map, OCIImageManifest.class, OCIImageManifest.mediaTypes());
        return map;
    }
}
