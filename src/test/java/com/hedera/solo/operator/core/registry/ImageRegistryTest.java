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
import com.hedera.solo.operator.library.container.oci.OCIImageIndex;
import com.hedera.solo.operator.library.container.oci.OCIImageManifest;
import com.hedera.solo.operator.model.commonspec.softwareversionsource.Layers;
import java.io.IOException;
import java.util.Optional;
import org.junit.jupiter.api.Test;

@WireMockTest
class ImageRegistryTest {
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
    }

    private static String mockUri(String port) {
        return "localhost:{port}/hedera-registry/consensus-node:0.56.0".replace("{port}", port);
    }

    private static String mockInvalidUri(String port) {
        return "localhost:{port}/invalid:0.56.0".replace("{port}", port);
    }

    private static String mockAuth(String port) {
        return "{\"auths\":{\"localhost:{port}\":{\"username\":\"user\",\"password\":\"pass\",\"auth\":\"dXNlcjpwYXNz\"}}}"
                .replace("{port}", port);
    }

    @Test
    void testFetchImageIndex(WireMockRuntimeInfo wmRuntimeInfo) throws IOException, InterruptedException {
        final String port = String.valueOf(wmRuntimeInfo.getHttpPort());
        final String imageUri = mockUri(port);
        final String invalidImageUri = mockInvalidUri(port);
        final String authJson = mockAuth(port);

        mockStubResponse(port);

        try (ImageRegistry registry = new ImageRegistry()) {
            final ImageCoordinates coordinates = ImageCoordinates.http(imageUri, authJson);
            final Optional<OCIImageIndex> resp = registry.fetchImageIndex(coordinates);
            assertThat(resp.isPresent()).isTrue();
            assertThat(resp.get().manifests.length).isGreaterThan(0);

            // failure scenario
            ImageRegistryException exception = assertThrows(
                    ImageRegistryException.class,
                    () -> registry.fetchImageIndex(ImageCoordinates.http(invalidImageUri, authJson)));
            assertThat(exception.getCause()).isInstanceOf(UnexpectedResponseException.class);
            assertThat(((UnexpectedResponseException) exception.getCause()).response.statusCode())
                    .isEqualTo(HTTP_NOT_FOUND);
        }
    }

    @Test
    void testFetchImageManifest(WireMockRuntimeInfo wmRuntimeInfo) throws IOException, InterruptedException {
        final String port = String.valueOf(wmRuntimeInfo.getHttpPort());
        final String imageUri = mockUri(port);
        final String invalidImageUri = mockInvalidUri(port);
        final String authJson = mockAuth(port);

        mockStubResponse(port);

        try (ImageRegistry registry = new ImageRegistry()) {
            final ImageCoordinates coordinates = ImageCoordinates.http(imageUri, authJson);
            final Optional<OCIImageManifest> imageManifest = registry.fetchImageManifest(
                    coordinates, Layers.Os.linux.getValue(), Layers.Architecture.amd64.getValue());
            assertThat(imageManifest.isPresent()).isTrue();
            assertThat(imageManifest.get().layers.length).isGreaterThan(0);

            // failure scenario
            ImageRegistryException exception = assertThrows(
                    ImageRegistryException.class,
                    () -> registry.fetchImageManifest(
                            ImageCoordinates.http(invalidImageUri, authJson),
                            Layers.Os.linux.getValue(),
                            Layers.Architecture.amd64.getValue()));
            assertThat(exception.getCause()).isInstanceOf(UnexpectedResponseException.class);
            assertThat(((UnexpectedResponseException) exception.getCause()).response.statusCode())
                    .isEqualTo(HTTP_NOT_FOUND);
        }
    }
}
