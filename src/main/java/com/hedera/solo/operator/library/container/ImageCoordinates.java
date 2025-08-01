// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.container;

import static com.fasterxml.jackson.databind.DeserializationFeature.FAIL_ON_UNKNOWN_PROPERTIES;
import static com.hedera.solo.operator.library.data.AsciiArmor.base64Encode;
import static java.util.Collections.emptySet;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.ObjectReader;
import java.io.IOException;
import java.net.URI;
import java.util.Collection;
import java.util.LinkedHashSet;
import java.util.Map;
import java.util.Objects;
import java.util.TreeMap;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

/** Represents the coordinates of an image, including its protocol, registry, name, tag, and digest. */
public class ImageCoordinates {
    private static final Pattern imagePattern =
            Pattern.compile("(?<registry>.*?)/(?<name>.*?)(:(?<tag>.*?))?(@(?<digest>.*))?");
    private static final ObjectReader objectReader =
            new ObjectMapper().disable(FAIL_ON_UNKNOWN_PROPERTIES).readerFor(Config.class);
    private static final Map<String, String> registries = registries();
    public final String protocol;
    public final String registry;
    public final String name;
    public final String tag;
    public final String digest;
    public final Collection<String> auths;

    /**
     * Constructs an ImageCoordinates object with specified protocol, registry, name, tag, and digest.
     *
     * @param protocol The protocol used to access the image registry.
     * @param registry The registry where the image is stored.
     * @param name The name of the image.
     * @param tag The tag associated with the image version (default: "latest").
     * @param digest The digest of the image for immutable references.
     * @param auths Authorization tokens for accessing private registries.
     */
    protected ImageCoordinates(
            final String protocol,
            final String registry,
            final String name,
            final String tag,
            final String digest,
            final Collection<String> auths) {
        this.protocol = protocol == null ? "https" : protocol;
        this.registry = registries.getOrDefault(registry, registry);
        this.name = name == null ? "" : name;
        this.tag = tag == null || tag.isBlank() ? "latest" : tag;
        this.digest = digest;
        this.auths = auths == null ? emptySet() : auths;
    }

    /**
     * Creates an ImageCoordinates object from a given image string and optional JSON configuration.
     *
     * @param image The image string containing registry, name, tag, and digest information.
     * @param json Optional JSON configuration for authentication details.
     * @return An ImageCoordinates object initialized from the provided image string
     */
    public static ImageCoordinates https(final String image, final String... json) {
        return from("https", image, json);
    }

    /**
     * Creates an ImageCoordinates object from a given protocol, image string, and optional JSON configuration.
     *
     * @param protocol The protocol used to access the image registry.
     * @param image The image string containing registry, name, tag, and digest information.
     * @param json Optional JSON configuration for authentication details.
     * @return An ImageCoordinates object initialized from the provided image string and protocol
     * @throws IllegalArgumentException if the image string does not match the expected format.
     */
    public static ImageCoordinates from(final String protocol, final String image, final String... json) {
        final Matcher matcher = imagePattern.matcher(image);
        if (!matcher.matches()) {
            throw new IllegalArgumentException("Failed to read image: " + image);
        }
        final String registry = matcher.group("registry");
        final String name = matcher.group("name");
        final String tag = matcher.group("tag");
        final String digest = matcher.group("digest");
        String img = registry + (name == null ? "" : "/" + name);
        Collection<String> auths = new LinkedHashSet<>();
        for (final String string : json) {
            try {
                objectReader.readValue(string, Config.class).auths.entrySet().stream()
                        .filter(e -> e.getKey().startsWith(registry))
                        .filter(e -> img.startsWith(e.getKey()))
                        .map(Map.Entry::getValue)
                        .map(Auth::auth)
                        .filter(Objects::nonNull)
                        .forEach(auths::add);
            } catch (IOException e) {
                throw new IllegalArgumentException("Failed to read json", e);
            }
        }
        return new ImageCoordinates(protocol, registry, name, tag, digest, auths);
    }

    /**
     * Returns the reference string of the image, which can be either the tag or the digest.
     *
     * @return The reference string of the image
     */
    public String reference() {
        return digest == null || digest.isBlank() ? tag : digest;
    }

    /**
     * Creates a new ImageCoordinates object with a different digest.
     *
     * @param newDigest The new digest for the image.
     * @return A new ImageCoordinates object with the specified digest
     */
    public ImageCoordinates digest(final String newDigest) {
        return new ImageCoordinates(protocol, registry, name, tag, newDigest, auths);
    }

    /**
     * Returns the URI to access the v2 API of the image registry.
     *
     * @return The URI to the v2 API of the image registry
     */
    public URI v2uri() {
        return URI.create(protocol + "://" + registry + "/v2/");
    }

    /**
     * Returns the URI to access a specific endpoint in the v2 API of the image registry.
     *
     * @param endpoint The specific endpoint within the v2 API.
     * @return The URI to the specified endpoint in the v2 API
     */
    public URI v2uri(final String endpoint) {
        return v2uri().resolve("./" + name + "/" + endpoint);
    }

    /** A configuration class for parsing JSON authentication details. */
    private static class Config {
        public Map<String, Auth> auths;
    }

    /** Represents authentication details for accessing private registries. */
    private static class Auth {
        public String username;
        public String password;
        public String email;
        public String auth;

        /**
         * Returns the base64-encoded authentication string.
         *
         * @return The base64-encoded authentication string
         */
        public String auth() {
            if (auth != null) {
                return auth;
            }
            if (username != null && password != null) {
                return base64Encode(username + ":" + password);
            }
            return null;
        }
    }

    /**
     * Initializes the map of default registries with case-insensitive keys.
     *
     * @return A map containing default registry mappings
     */
    private static Map<String, String> registries() {
        final Map<String, String> map = new TreeMap<>(String.CASE_INSENSITIVE_ORDER);
        map.put("docker.io", "registry-1.docker.io");
        return map;
    }

    @Override
    public boolean equals(final Object o) {
        if (this == o) return true;
        if (o == null || getClass() != o.getClass()) return false;
        final ImageCoordinates that = (ImageCoordinates) o;
        return Objects.equals(protocol, that.protocol)
                && Objects.equals(registry, that.registry)
                && Objects.equals(name, that.name)
                && Objects.equals(auths, that.auths);
    }

    @Override
    public int hashCode() {
        return Objects.hash(protocol, registry, name, auths);
    }
}
