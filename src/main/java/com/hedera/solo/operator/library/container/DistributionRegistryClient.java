// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.container;

import static com.fasterxml.jackson.databind.DeserializationFeature.FAIL_ON_UNKNOWN_PROPERTIES;
import static java.net.HttpURLConnection.HTTP_OK;
import static java.net.HttpURLConnection.HTTP_UNAUTHORIZED;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.ObjectReader;
import com.hedera.solo.operator.library.container.oci.OCIContentDescriptor;
import com.hedera.solo.operator.library.container.oci.OCIManifest;
import com.hedera.solo.operator.library.generic.LRUCache;
import java.io.IOException;
import java.net.URI;
import java.net.URLEncoder;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.charset.StandardCharsets;
import java.util.Arrays;
import java.util.Collections;
import java.util.Map;
import java.util.Optional;
import java.util.Set;
import java.util.TreeMap;
import java.util.TreeSet;
import java.util.function.Function;
import java.util.regex.Matcher;
import java.util.regex.Pattern;
import java.util.stream.Collectors;

/**
 * Client for distribution registry api v2
 *
 * @see <a href="https://distribution.github.io/distribution/spec/api/">Distribution Registry: HTTP API V2</a>
 * @see <a href="https://github.com/opencontainers/distribution-spec/blob/main/spec.md">Open Container Initiative
 *     Distribution Specification</a>
 */
public class DistributionRegistryClient implements AutoCloseable {
    protected static final String[] manifestMediaTypes = OCIManifest.mediaTypes();
    protected final AuthClient client;

    public DistributionRegistryClient() {
        this(100, HttpClient.newBuilder().followRedirects(HttpClient.Redirect.NORMAL));
    }

    public DistributionRegistryClient(final int cacheSize, final HttpClient.Builder builder) {
        client = new AuthClient(cacheSize, builder.build());
    }

    /**
     * Fetch manifest corresponding to image coordinates and media types, if provided.
     *
     * @param coordinates to image
     * @param mediaTypes are optional and defaults to all recognized manifest media types.
     */
    public ManifestResponse fetchManifest(final ImageCoordinates coordinates, final String... mediaTypes)
            throws IOException, InterruptedException {
        return new ManifestResponse(get(
                coordinates,
                "manifests/" + coordinates.reference,
                mediaTypes.length == 0 ? manifestMediaTypes : mediaTypes));
    }

    /**
     * Verify given layer exists at repository.
     *
     * @param coordinates to image
     * @param layer to check
     */
    public boolean layerExists(final ImageCoordinates coordinates, final OCIContentDescriptor layer)
            throws IOException, InterruptedException {
        return head(coordinates, "blobs/" + layer.digest, layer.mediaType).statusCode() == HTTP_OK;
    }

    protected HttpResponse<byte[]> get(
            final ImageCoordinates coordinates, final String endpoint, final String... mediaTypes)
            throws IOException, InterruptedException {
        return client.send(coordinates, HttpRequest.Builder::GET, endpoint, mediaTypes);
    }

    protected HttpResponse<byte[]> head(
            final ImageCoordinates coordinates, final String endpoint, final String... mediaTypes)
            throws IOException, InterruptedException {
        return client.send(coordinates, HttpRequest.Builder::HEAD, endpoint, mediaTypes);
    }

    @Override
    public void close() {
        client.close();
    }

    /**
     * Client for handling authentication.
     *
     * @see <a href="https://distribution.github.io/distribution/spec/auth/token/">Token Authentication
     *     Specification</a>
     */
    protected static class AuthClient implements AutoCloseable {
        protected static final Pattern authChallenge = Pattern.compile("(?<key>\\S+?)\\s*=\\s*\"(?<val>\\S+?)\"\\s*,?");
        protected static final AuthResponse invalidAuthResponse = new AuthResponse();
        protected static final Set<String> necessaryAttributes = attributes();
        protected final Map<ImageCoordinates, AuthResponse> cache;
        protected final HttpClient client;

        public AuthClient(final int cacheSize, final HttpClient client) {
            cache = Collections.synchronizedMap(new LRUCache<>(cacheSize));
            this.client = client;
        }

        public HttpResponse<byte[]> send(
                final ImageCoordinates coordinates,
                final Function<HttpRequest.Builder, HttpRequest.Builder> function,
                final String endpoint,
                final String... mediaTypes)
                throws IOException, InterruptedException {
            AuthResponse authResponse = cache.getOrDefault(coordinates, invalidAuthResponse);
            if (authResponse == invalidAuthResponse) {
                HttpResponse<byte[]> response = send(null, coordinates.v2uri(), HttpRequest.Builder::GET);
                authResponse = switch (response.statusCode()) {
                    case HTTP_OK -> null;
                    case HTTP_UNAUTHORIZED -> authenticate(coordinates, response);
                    default -> throw new UnexpectedResponseException(response);};
            }
            URI uri = coordinates.v2uri(endpoint);
            HttpResponse<byte[]> response = send(authResponse, uri, function, mediaTypes);
            if (response.statusCode() == HTTP_UNAUTHORIZED) {
                // Cached token may be expired or registry may enforce single-use token policy.
                authenticate(coordinates, response);
                response = send(authResponse, uri, function, mediaTypes);
            }
            if (response.statusCode() != HTTP_OK) {
                throw new UnexpectedResponseException(response);
            }
            cache.put(coordinates, authResponse);
            return response;
        }

        protected HttpResponse<byte[]> send(
                final AuthResponse authResponse,
                final URI uri,
                final Function<HttpRequest.Builder, HttpRequest.Builder> function,
                final String... mediaTypes)
                throws IOException, InterruptedException {
            HttpRequest.Builder builder = HttpRequest.newBuilder(uri);
            Arrays.stream(mediaTypes).forEach(type -> builder.header("Accept", type));
            if (authResponse != null) {
                builder.header("Authorization", "Bearer " + authResponse.token());
            }
            return client.send(function.apply(builder).build(), HttpResponse.BodyHandlers.ofByteArray());
        }

        protected AuthResponse authenticate(
                final ImageCoordinates coordinates, final HttpResponse<byte[]> unauthorizedResponse)
                throws IOException, InterruptedException {
            Optional<String> header = unauthorizedResponse.headers().firstValue("Www-Authenticate");
            Matcher matcher = authChallenge.matcher(header.orElseThrow());
            Map<String, String> attributes = new TreeMap<>(String.CASE_INSENSITIVE_ORDER);
            while (matcher.find()) {
                attributes.put(matcher.group("key"), matcher.group("val"));
            }
            String realm = attributes.get("realm");
            attributes.keySet().retainAll(necessaryAttributes);
            attributes.putIfAbsent("scope", "repository:" + coordinates.name + ":pull");
            attributes.putIfAbsent("service", coordinates.registry);
            URI uri = URI.create(realm + "?"
                    + attributes.entrySet().stream()
                            .map(e -> URLEncoder.encode(e.getKey(), StandardCharsets.UTF_8) + "="
                                    + URLEncoder.encode(e.getValue(), StandardCharsets.UTF_8))
                            .collect(Collectors.joining("&")));
            HttpRequest.Builder builder = HttpRequest.newBuilder(uri).GET();
            if (coordinates.auth != null) {
                builder.header("Authorization", "Basic " + coordinates.auth);
            }
            HttpResponse<byte[]> tokenResponse = client.send(builder.build(), HttpResponse.BodyHandlers.ofByteArray());
            if (tokenResponse.statusCode() != HTTP_OK) {
                throw new UnexpectedResponseException(tokenResponse);
            }
            return AuthResponse.from(tokenResponse.body());
        }

        private static Set<String> attributes() {
            Set<String> attributes = new TreeSet<>(String.CASE_INSENSITIVE_ORDER);
            attributes.addAll(Set.of("scope", "service"));
            return attributes;
        }

        @Override
        public void close() {
            client.close();
        }
    }

    /**
     * Authorization token successful response
     *
     * @see <a href="https://distribution.github.io/distribution/spec/auth/token/#token-response-fields">Token
     *     Authentication Specification: Token Response Fields</a>
     * @see <a href="https://datatracker.ietf.org/doc/html/rfc6749#section-5.1">The OAuth 2.0 Authorization Framework:
     *     Issuing an Access Token: Successful Response</a>
     */
    protected static class AuthResponse {
        protected static final ObjectReader objectReader =
                new ObjectMapper().disable(FAIL_ON_UNKNOWN_PROPERTIES).readerFor(AuthResponse.class);
        public String token;
        public String access_token;
        public String token_type;
        public Long expires_in;
        public String issued_at;
        public String refresh_token;
        public String scope;

        public static AuthResponse from(final byte[] content) throws IOException {
            return objectReader.readValue(content);
        }

        public String token() {
            return token != null ? token : access_token;
        }
    }
}
