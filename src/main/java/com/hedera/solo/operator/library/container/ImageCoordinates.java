// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.container;

import static com.fasterxml.jackson.databind.DeserializationFeature.FAIL_ON_UNKNOWN_PROPERTIES;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.ObjectReader;
import java.io.IOException;
import java.net.URI;
import java.nio.charset.StandardCharsets;
import java.util.Base64;
import java.util.Map;
import java.util.Objects;
import java.util.TreeMap;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

public class ImageCoordinates {
    private static final Pattern imagePattern = Pattern.compile("(?<registry>.*?)/(?<name>.*?)(:(?<reference>.*))?");
    private static final ObjectReader objectReader =
            new ObjectMapper().disable(FAIL_ON_UNKNOWN_PROPERTIES).readerFor(Config.class);
    private static final Map<String, String> registries = registries();
    public final String protocol;
    public final String registry;
    public final String name;
    public final String reference;
    public final String auth;

    public ImageCoordinates(
            final String protocol,
            final String registry,
            final String name,
            final String reference,
            final String auth) {
        this.protocol = protocol;
        this.registry = registries.getOrDefault(registry, registry);
        this.name = name;
        this.reference = reference == null || reference.isBlank() ? "latest" : reference;
        this.auth = auth;
    }

    public static ImageCoordinates http(final String image) {
        return http(image, null);
    }

    public static ImageCoordinates http(final String image, final String json) {
        return from("http://", image, json);
    }

    public static ImageCoordinates https(final String image) {
        return https(image, null);
    }

    public static ImageCoordinates https(final String image, final String json) {
        return from("https://", image, json);
    }

    public static ImageCoordinates from(final String protocol, final String image, final String json) {
        final Matcher matcher = imagePattern.matcher(image);
        if (!matcher.matches()) {
            throw new IllegalArgumentException("Failed to read image: " + image);
        }
        final String registry = matcher.group("registry");
        final String name = matcher.group("name");
        final String reference = matcher.group("reference");
        String auth = null;
        if (json != null) {
            Config config;
            try {
                config = objectReader.readValue(json);
            } catch (IOException e) {
                throw new IllegalArgumentException("Failed to read json", e);
            }
            final Auth entry = config.auths.get(registry);
            if (entry == null || entry.auth == null && (entry.username == null || entry.password == null)) {
                throw new IllegalArgumentException("Failed to read auth");
            }
            auth = entry.auth != null
                    ? entry.auth
                    : Base64.getEncoder()
                            .encodeToString((entry.username + ":" + entry.password).getBytes(StandardCharsets.UTF_8));
        }
        return new ImageCoordinates(protocol, registry, name, reference, auth);
    }

    public ImageCoordinates reference(final String reference) {
        return new ImageCoordinates(protocol, registry, name, reference, auth);
    }

    public URI v2uri() {
        return URI.create(protocol + registry + "/v2/");
    }

    public URI v2uri(final String endpoint) {
        return v2uri().resolve(name + "/" + endpoint);
    }

    private static class Config {
        public Map<String, Auth> auths;
    }

    private static class Auth {
        public String username;
        public String password;
        public String email;
        public String auth;
    }

    private static Map<String, String> registries() {
        final Map<String, String> registries = new TreeMap<>(String.CASE_INSENSITIVE_ORDER);
        registries.put("docker.io", "registry-1.docker.io");
        return registries;
    }

    @Override
    public boolean equals(final Object o) {
        if (this == o) return true;
        if (o == null || getClass() != o.getClass()) return false;
        final ImageCoordinates that = (ImageCoordinates) o;
        return Objects.equals(registry, that.registry)
                && Objects.equals(name, that.name)
                && Objects.equals(auth, that.auth);
    }

    @Override
    public int hashCode() {
        return Objects.hash(registry, name, auth);
    }
}
