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
import edu.umd.cs.findbugs.annotations.NonNull;
import jakarta.enterprise.context.Dependent;
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
import java.util.Objects;
import java.util.Optional;
import java.util.Set;
import java.util.TreeMap;
import java.util.TreeSet;
import java.util.function.Function;
import java.util.regex.Matcher;
import java.util.regex.Pattern;
import java.util.stream.Collectors;

/**
 * Client for distribution registry api v2.
 *
 * @see <a href="https://distribution.github.io/distribution/spec/api/">Distribution Registry: HTTP API V2</a>
 * @see <a href="https://github.com/opencontainers/distribution-spec/blob/main/spec.md">Open Container Initiative
 *     Distribution Specification</a>
 */
@Dependent
public class DistributionRegistryClient implements AutoCloseable {
    protected static final String[] manifestMediaTypes = OCIManifest.mediaTypes();
    protected final AuthClient client;

    /** Default constructor initializes the client with a default cache size and HTTP client builder. */
    public DistributionRegistryClient() {
        this(100, HttpClient.newBuilder().followRedirects(HttpClient.Redirect.NORMAL));
    }

    /**
     * Constructor initializes the client with a specified cache size and HTTP client builder.
     *
     * @param cacheSize the size of the cache
     * @param builder the HTTP client builder
     */
    public DistributionRegistryClient(final int cacheSize, final HttpClient.Builder builder) {
        client = new AuthClient(cacheSize, builder.build());
    }

    /**
     * Fetch manifest corresponding to image coordinates and media types, if provided.
     *
     * @param coordinates to image
     * @param mediaTypes are optional and defaults to all recognized manifest media types.
     * @return the manifest response for the given image coordinates and media types
     * @throws IOException if an I/O error occurs when sending or receiving
     * @throws InterruptedException if the operation is interrupted
     */
    public ManifestResponse fetchManifest(@NonNull final ImageCoordinates coordinates, final String... mediaTypes)
            throws IOException, InterruptedException {
        Objects.requireNonNull(coordinates, "coordinates cannot be null");
        return new ManifestResponse(
                coordinates,
                get(
                        coordinates,
                        "manifests/" + coordinates.reference(),
                        mediaTypes == null || mediaTypes.length == 0 ? manifestMediaTypes : mediaTypes));
    }

    /**
     * Verify given layer exists at repository.
     *
     * @param coordinates to image
     * @param layer to check
     * @return true if layer exists, false otherwise
     * @throws IOException if an I/O error occurs when sending or receiving
     * @throws InterruptedException if the operation is interrupted
     */
    public boolean layerExists(@NonNull final ImageCoordinates coordinates, @NonNull final OCIContentDescriptor layer)
            throws IOException, InterruptedException {
        Objects.requireNonNull(coordinates, "coordinates cannot be null");
        Objects.requireNonNull(layer, "layer cannot be null");
        Objects.requireNonNull(layer.digest, "layer digest cannot be null");
        Objects.requireNonNull(layer.mediaType, "layer mediaType cannot be null");
        return head(coordinates, "blobs/" + layer.digest, layer.mediaType).statusCode() == HTTP_OK;
    }

    /**
     * Sends a GET request to the registry.
     *
     * @param coordinates to image
     * @param endpoint to append to the base URI
     * @param mediaTypes to accept in the response
     * @return the HTTP response containing the resource as a byte array
     * @throws IOException if an I/O error occurs when sending or receiving
     * @throws InterruptedException if the operation is interrupted
     */
    protected HttpResponse<byte[]> get(
            final ImageCoordinates coordinates, final String endpoint, final String... mediaTypes)
            throws IOException, InterruptedException {
        return client.send(coordinates, HttpRequest.Builder::GET, endpoint, mediaTypes);
    }

    /**
     * Sends a HEAD request to the registry.
     *
     * @param coordinates to image
     * @param endpoint to append to the base URI
     * @param mediaTypes to accept in the response
     * @return the HTTP response containing the resource as a byte array
     * @throws IOException if an I/O error occurs when sending or receiving
     * @throws InterruptedException if the operation is interrupted
     */
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
        protected static final Set<String> necessaryAttributes = attributes();
        protected final Map<ImageCoordinates, Optional<AuthResponse>> cache;
        protected final HttpClient client;

        /**
         * Constructor initializes the AuthClient with a specified cache size and HTTP client.
         *
         * @param cacheSize the size of the cache
         * @param client the HTTP client
         */
        public AuthClient(final int cacheSize, final HttpClient client) {
            cache = Collections.synchronizedMap(new LRUCache<>(cacheSize));
            this.client = client;
        }

        /**
         * Sends a request to the registry with optional authentication and returns the response.
         *
         * @param coordinates to image
         * @param function to apply to HttpRequest.Builder
         * @param endpoint to append to the base URI
         * @param mediaTypes to accept in the response
         * @return the HTTP response containing the resource as a byte array
         * @throws IOException if an I/O error occurs when sending or receiving
         * @throws InterruptedException if the operation is interrupted
         * @throws UnexpectedResponseException if the response status code is not HTTP_OK or HTTP_UNAUTHORIZED
         */
        public HttpResponse<byte[]> send(
                final ImageCoordinates coordinates,
                final Function<HttpRequest.Builder, HttpRequest.Builder> function,
                final String endpoint,
                final String... mediaTypes)
                throws IOException, InterruptedException {
            Optional<AuthResponse> authResponse = cache.get(coordinates);
            if (authResponse == null) {
                final HttpResponse<byte[]> response =
                        send(Optional.empty(), coordinates.v2uri(), HttpRequest.Builder::GET);
                authResponse = switch (response.statusCode()) {
                    case HTTP_OK -> Optional.empty();
                    case HTTP_UNAUTHORIZED -> Optional.of(authenticate(coordinates, response));
                    default -> throw new UnexpectedResponseException(response);};
            }
            final URI uri = coordinates.v2uri(endpoint);
            HttpResponse<byte[]> response = send(authResponse, uri, function, mediaTypes);
            if (response.statusCode() == HTTP_UNAUTHORIZED) {
                // Cached token may be expired or registry may enforce single-use token policy.
                authResponse = Optional.of(authenticate(coordinates, response));
                response = send(authResponse, uri, function, mediaTypes);
            }
            if (response.statusCode() != HTTP_OK) {
                throw new UnexpectedResponseException(response);
            }
            cache.put(coordinates, authResponse);
            return response;
        }

        /**
         * Sends a request to the registry with authentication and returns the response.
         *
         * @param authResponse optional authentication response
         * @param uri the URI to send the request to
         * @param function to apply to HttpRequest.Builder
         * @param mediaTypes to accept in the response
         * @return the HTTP response containing the resource as a byte array
         * @throws IOException if an I/O error occurs when sending or receiving
         * @throws InterruptedException if the operation is interrupted
         */
        protected HttpResponse<byte[]> send(
                final Optional<AuthResponse> authResponse,
                final URI uri,
                final Function<HttpRequest.Builder, HttpRequest.Builder> function,
                final String... mediaTypes)
                throws IOException, InterruptedException {
            final HttpRequest.Builder builder = HttpRequest.newBuilder(uri);
            Arrays.stream(mediaTypes).forEach(type -> builder.header("Accept", type));
            authResponse.ifPresent(auth -> builder.header("Authorization", "Bearer " + auth.token()));
            return client.send(function.apply(builder).build(), HttpResponse.BodyHandlers.ofByteArray());
        }

        /**
         * Authenticates the request to the registry using the provided challenge header.
         *
         * @param coordinates to image
         * @param unauthorizedResponse response with 401 Unauthorized status
         * @return an AuthResponse containing the authentication token
         * @throws IOException if an I/O error occurs when sending or receiving
         * @throws InterruptedException if the operation is interrupted
         */
        protected AuthResponse authenticate(
                final ImageCoordinates coordinates, final HttpResponse<byte[]> unauthorizedResponse)
                throws IOException, InterruptedException {
            // Example header from: https://distribution.github.io/distribution/spec/auth/token/
            // Www-Authenticate: Bearer
            // realm="https://auth.docker.io/token",service="registry.docker.io",scope="repository:samalba/my-app:pull,push"
            final Optional<String> header = unauthorizedResponse.headers().firstValue("Www-Authenticate");
            final Matcher matcher = authChallenge.matcher(header.orElseThrow());
            final Map<String, String> attributes = new TreeMap<>(String.CASE_INSENSITIVE_ORDER);
            while (matcher.find()) {
                attributes.put(matcher.group("key"), matcher.group("val"));
            }
            final String realm = attributes.get("realm");
            attributes.keySet().retainAll(necessaryAttributes);
            attributes.putIfAbsent("scope", "repository:" + coordinates.name + ":pull");
            attributes.putIfAbsent("service", coordinates.registry);
            final URI uri = URI.create(realm + "?"
                    + attributes.entrySet().stream()
                            .map(e -> URLEncoder.encode(e.getKey(), StandardCharsets.UTF_8) + "="
                                    + URLEncoder.encode(e.getValue(), StandardCharsets.UTF_8))
                            .collect(Collectors.joining("&")));
            for (String auth : coordinates.auths) {
                AuthResponse authResponse = authenticate(uri, auth);
                if (authResponse != null) {
                    return authResponse;
                }
            }
            return authenticate(uri, null);
        }

        /**
         * Authenticates with the registry using the provided URI and authorization header.
         *
         * @param uri the URI to authenticate against
         * @param auth the base64-encoded authorization string (username:password)
         * @return an AuthResponse containing the authentication details
         * @throws IOException if an I/O error occurs when sending or receiving
         * @throws InterruptedException if the operation is interrupted
         * @throws UnexpectedResponseException if the response status code is not HTTP_OK or HTTP_UNAUTHORIZED
         */
        protected AuthResponse authenticate(final URI uri, final String auth) throws IOException, InterruptedException {
            final HttpRequest.Builder builder = HttpRequest.newBuilder(uri).GET();
            if (auth != null) {
                builder.header("Authorization", "Basic " + auth);
            }
            final HttpResponse<byte[]> tokenResponse =
                    client.send(builder.build(), HttpResponse.BodyHandlers.ofByteArray());
            if (auth != null && tokenResponse.statusCode() == HTTP_UNAUTHORIZED) {
                return null;
            }
            if (tokenResponse.statusCode() != HTTP_OK) {
                throw new UnexpectedResponseException(tokenResponse);
            }
            return AuthResponse.from(tokenResponse.body());
        }

        /**
         * Creates a set of necessary attributes for authentication.
         *
         * @return a set containing the necessary attributes "scope" and "service"
         */
        private static Set<String> attributes() {
            final Set<String> attributes = new TreeSet<>(String.CASE_INSENSITIVE_ORDER);
            attributes.addAll(Set.of("scope", "service"));
            return attributes;
        }

        @Override
        public void close() {
            client.close();
        }
    }

    /**
     * Authorization token successful response.
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

        /**
         * Creates an AuthResponse from a JSON byte array.
         *
         * @param content the JSON byte array
         * @return the AuthResponse instance
         * @throws IOException if an I/O error occurs while deserializing
         */
        public static AuthResponse from(final byte[] content) throws IOException {
            return objectReader.readValue(content);
        }

        /**
         * Gets the token value, preferring 'token' field if present.
         *
         * @return the token value
         */
        public String token() {
            return token != null ? token : access_token;
        }
    }
}
