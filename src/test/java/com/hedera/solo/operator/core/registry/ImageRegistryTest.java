// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.core.registry;

import static com.github.tomakehurst.wiremock.client.WireMock.get;
import static com.github.tomakehurst.wiremock.client.WireMock.head;
import static com.github.tomakehurst.wiremock.client.WireMock.notFound;
import static com.github.tomakehurst.wiremock.client.WireMock.ok;
import static com.github.tomakehurst.wiremock.client.WireMock.stubFor;
import static com.github.tomakehurst.wiremock.client.WireMock.unauthorized;
import static com.github.tomakehurst.wiremock.client.WireMock.urlEqualTo;
import static com.github.tomakehurst.wiremock.client.WireMock.urlMatching;
import static java.net.HttpURLConnection.HTTP_NOT_FOUND;
import static org.assertj.core.api.Assertions.assertThat;
import static org.junit.jupiter.api.Assertions.assertThrows;

import com.github.tomakehurst.wiremock.junit5.WireMockRuntimeInfo;
import com.github.tomakehurst.wiremock.junit5.WireMockTest;
import com.hedera.solo.operator.library.container.ImageCoordinates;
import com.hedera.solo.operator.library.container.UnexpectedResponseException;
import com.hedera.solo.operator.library.container.oci.OCIContentDescriptor;
import com.hedera.solo.operator.library.container.oci.OCIImageIndex;
import com.hedera.solo.operator.library.container.oci.OCIImageManifest;
import com.hedera.solo.operator.model.commonspec.ImageVerificationSpec;
import java.util.List;
import org.junit.jupiter.api.Test;

/**
 * A test class for the {@code ImageRegistry} implementation, focusing on verifying the behavior of image registry
 * operations using WireMock for HTTP mocking.
 */
@WireMockTest
class ImageRegistryTest {
    /**
     * Mocks a response from the server for various HTTP methods to simulate different scenarios.
     *
     * @param port The port number used by WireMock for the mock server.
     */
    private static void mockStubResponse(String port) {
        stubFor(get("/v2/")
                .willReturn(unauthorized()
                        .withHeader(
                                "www-authenticate",
                                "Bearer realm=\"http://localhost:{port}/v2/token\"".replace("{port}", port))));

        stubFor(get(urlMatching("/v2/token?.*"))
                .willReturn(ok().withHeader("Content-Type", "application/json").withBody("{\"token\":\"jwt\"}")));

        stubFor(get("/v2/invalid/manifests/0.56.0").willReturn(notFound())); // for failure scenario

        stubFor(
                get("/v2/hedera-registry/consensus-node/manifests/0.56.0")
                        .willReturn(
                                ok().withHeader("Content-Type", "application/vnd.oci.image.index.v1+json")
                                        .withBody(
                                                "{\"manifests\":[{\"mediaType\":\"application/vnd.oci.image.manifest.v1+json\",\"digest\":\"hash:123\",\"platform\":{\"os\":\"linux\",\"architecture\":\"amd64\"}}]}")));

        stubFor(
                get("/v2/hedera-registry/consensus-node/manifests/hash:123")
                        .willReturn(
                                ok().withHeader("Content-Type", "application/vnd.oci.image.manifest.v1+json")
                                        .withBody(
                                                "{\"layers\":[{\"mediaType\":\"application/vnd.oci.image.layer.v1.tar+gzip\",\"digest\":\"hash:1\"}]}")));

        stubFor(head(urlEqualTo("/v2/hedera-registry/consensus-node/blobs/hash:1"))
                .willReturn(ok()));
        stubFor(head(urlEqualTo("/v2/hedera-registry/consensus-node/blobs/hash:2"))
                .willReturn(ok()));
    }

    /**
     * Constructs a mock URI string based on the provided port number.
     *
     * @param port The port number to be used in the URI.
     * @return A URI string with the placeholder {port} replaced by the actual port number
     */
    private static String mockUri(String port) {
        return "localhost:{port}/hedera-registry/consensus-node:0.56.0".replace("{port}", port);
    }

    /**
     * Constructs a mock URI string for an invalid image based on the provided port number.
     *
     * @param port The port number to be used in the URI.
     * @return A URI string with the placeholder {port} replaced by the actual port number, indicating an invalid
     *     endpoint.
     */
    private static String mockInvalidUri(String port) {
        return "localhost:{port}/invalid:0.56.0".replace("{port}", port);
    }

    /**
     * Constructs a mock authentication JSON string based on the provided port number.
     *
     * @param port The port number to be used in the URI placeholder within the JSON string.
     * @return A JSON string with the placeholder {port} replaced by the actual port number, representing authentication
     *     details for an image registry.
     */
    private static String mockAuth(String port) {
        return "{\"auths\":{\"localhost:{port}\":{\"username\":\"user\",\"password\":\"pass\",\"auth\":\"dXNlcjpwYXNz\"}}}"
                .replace("{port}", port);
    }

    @Test
    void fetchImageIndex(WireMockRuntimeInfo wmRuntimeInfo) {
        final String port = String.valueOf(wmRuntimeInfo.getHttpPort());
        final String imageUri = mockUri(port);
        final String invalidImageUri = mockInvalidUri(port);
        final String authJson = mockAuth(port);

        mockStubResponse(port);

        try (ImageRegistry registry = new ImageRegistry()) {
            final ImageCoordinates coordinates = imageCoordinatesHttp(imageUri, authJson);
            final OCIImageIndex resp = registry.fetchImageIndex(coordinates);
            assertThat(resp).isNotNull();
            assertThat(resp.manifests.length).isGreaterThan(0);

            // failure scenario
            RecordNotFoundException exception = assertThrows(
                    RecordNotFoundException.class,
                    () -> registry.fetchImageIndex(imageCoordinatesHttp(invalidImageUri, authJson)));
            assertThat(exception.getCause()).isInstanceOf(UnexpectedResponseException.class);
            assertThat(((UnexpectedResponseException) exception.getCause()).response.statusCode())
                    .isEqualTo(HTTP_NOT_FOUND);
        }
    }

    @Test
    void fetchImageManifest(WireMockRuntimeInfo wmRuntimeInfo) {
        final String port = String.valueOf(wmRuntimeInfo.getHttpPort());
        final String imageUri = mockUri(port);
        final String invalidImageUri = mockInvalidUri(port);
        final String authJson = mockAuth(port);

        mockStubResponse(port);

        try (ImageRegistry registry = new ImageRegistry()) {
            final ImageCoordinates coordinates = imageCoordinatesHttp(imageUri, authJson);
            final OCIImageManifest imageManifest = registry.fetchImageManifest(
                    coordinates,
                    ImageVerificationSpec.Os.linux.getValue(),
                    ImageVerificationSpec.Architecture.amd64.getValue());
            assertThat(imageManifest).isNotNull();
            assertThat(imageManifest.layers.length).isGreaterThan(0);

            // failure scenario
            RecordNotFoundException exception = assertThrows(
                    RecordNotFoundException.class,
                    () -> registry.fetchImageManifest(
                            imageCoordinatesHttp(invalidImageUri, authJson),
                            ImageVerificationSpec.Os.linux.getValue(),
                            ImageVerificationSpec.Architecture.amd64.getValue()));
            assertThat(exception.getCause()).isInstanceOf(UnexpectedResponseException.class);
            assertThat(((UnexpectedResponseException) exception.getCause()).response.statusCode())
                    .isEqualTo(HTTP_NOT_FOUND);
        }
    }

    @Test
    void isValidLayers(WireMockRuntimeInfo wmRuntimeInfo) {
        final String port = String.valueOf(wmRuntimeInfo.getHttpPort());
        final String imageUri = mockUri(port);
        final String authJson = mockAuth(port);

        mockStubResponse(port);

        try (ImageRegistry registry = new ImageRegistry()) {
            final ImageCoordinates coordinates = imageCoordinatesHttp(imageUri, authJson);
            final OCIImageManifest manifest = new OCIImageManifest();
            manifest.layers = new OCIContentDescriptor[] {
                OCIContentDescriptor.builder()
                        .withDigest("hash:1")
                        .withMediaType(OCIImageManifest.mediaTypes()[0])
                        .build(),
                OCIContentDescriptor.builder()
                        .withDigest("hash:2")
                        .withMediaType(OCIImageManifest.mediaTypes()[0])
                        .build(),
            };

            // incorrect number of layers
            assertThat(registry.hasValidLayers(coordinates, manifest, List.of("hash:11")))
                    .isFalse();

            // incorrect orders of layers
            assertThat(registry.hasValidLayers(coordinates, manifest, List.of("hash:2", "hash:1")))
                    .isFalse();

            // correct orders of layers
            assertThat(registry.hasValidLayers(coordinates, manifest, List.of("hash:1", "hash:2")))
                    .isTrue();
        }
    }

    /**
     * Helper method to create an {@link ImageCoordinates} object from the provided image URI and JSON data.
     *
     * @param image The image URI or path.
     * @param json A JSON string representing authentication details (e.g., username, password).
     * @return An {@code ImageCoordinates} instance populated with the given parameters
     */
    ImageCoordinates imageCoordinatesHttp(final String image, final String json) {
        return ImageCoordinates.from("http", image, json);
    }
}
