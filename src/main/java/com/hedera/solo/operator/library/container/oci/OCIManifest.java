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

    /**
     * Creates an instance of {@code OCIManifest} based on the media type and byte array.
     *
     * @param mediaType the media type of the manifest
     * @param bytes the byte array representation of the manifest
     * @return the instantiated {@code OCIManifest} or null if no matching constructor is found
     */
    public static OCIManifest from(final String mediaType, final byte[] bytes) {
        return Optional.ofNullable(OCIManifest.constructors.get(mediaType))
                .map(ctor -> ctor.apply(bytes))
                .orElse(null);
    }

    /**
     * Returns the array of supported media types for {@code OCIManifest}.
     *
     * @return an array of strings representing the media types
     */
    public static String[] mediaTypes() {
        return constructors.keySet().toArray(String[]::new);
    }

    /**
     * Creates a deserialization function for a given class.
     *
     * @param aClass the class type to deserialize into
     * @param <T> the type of the class
     * @return a function that takes byte array and returns an instance of the class
     */
    private static <T extends OCIManifest> Function<byte[], T> to(final Class<T> aClass) {
        return bytes -> {
            try {
                return objectReader.readValue(bytes, aClass);
            } catch (IOException e) {
                throw new RuntimeException(e);
            }
        };
    }

    /**
     * Adds constructors for a given class and media types to the map.
     *
     * @param map the map of constructors
     * @param aClass the class type to add constructors for
     * @param mediaTypes the array of media types associated with the class
     * @param <T> the type of the class
     */
    private static <T extends OCIManifest> void add(
            final Map<String, Function<byte[], ? extends OCIManifest>> map,
            final Class<T> aClass,
            final String... mediaTypes) {
        Arrays.stream(mediaTypes).forEach(mediaType -> map.put(mediaType, to(aClass)));
    }

    /**
     * Initializes the constructors map with mappings from media types to deserialization functions.
     *
     * @return a map of media types to deserialization functions
     */
    private static Map<String, Function<byte[], ? extends OCIManifest>> constructors() {
        final Map<String, Function<byte[], ? extends OCIManifest>> map = new HashMap<>();
        add(map, OCIImageIndex.class, OCIImageIndex.mediaTypes());
        add(map, OCIImageManifest.class, OCIImageManifest.mediaTypes());
        return map;
    }
}
