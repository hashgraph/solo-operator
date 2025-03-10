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

    public static ImageCoordinates https(final String image, final String... json) {
        return from("https", image, json);
    }

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

    public String reference() {
        return digest == null || digest.isBlank() ? tag : digest;
    }

    public ImageCoordinates digest(final String digest) {
        return new ImageCoordinates(protocol, registry, name, tag, digest, auths);
    }

    public URI v2uri() {
        return URI.create(protocol + "://" + registry + "/v2/");
    }

    public URI v2uri(final String endpoint) {
        return v2uri().resolve("./" + name + "/" + endpoint);
    }

    private static class Config {
        public Map<String, Auth> auths;
    }

    private static class Auth {
        public String username;
        public String password;
        public String email;
        public String auth;

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
