// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.unit;

import static com.github.tomakehurst.wiremock.client.WireMock.equalTo;
import static com.github.tomakehurst.wiremock.client.WireMock.get;
import static com.github.tomakehurst.wiremock.client.WireMock.head;
import static com.github.tomakehurst.wiremock.client.WireMock.not;
import static com.github.tomakehurst.wiremock.client.WireMock.ok;
import static com.github.tomakehurst.wiremock.client.WireMock.reset;
import static com.github.tomakehurst.wiremock.client.WireMock.stubFor;
import static com.github.tomakehurst.wiremock.client.WireMock.unauthorized;
import static com.github.tomakehurst.wiremock.client.WireMock.urlEqualTo;
import static com.github.tomakehurst.wiremock.client.WireMock.urlMatching;
import static java.net.HttpURLConnection.HTTP_UNAUTHORIZED;
import static org.assertj.core.api.Assertions.assertThat;
import static org.junit.jupiter.api.Assertions.assertThrows;

import com.github.tomakehurst.wiremock.junit5.WireMockRuntimeInfo;
import com.github.tomakehurst.wiremock.junit5.WireMockTest;
import com.hedera.solo.operator.library.container.DistributionRegistryClient;
import com.hedera.solo.operator.library.container.ImageCoordinates;
import com.hedera.solo.operator.library.container.ManifestResponse;
import com.hedera.solo.operator.library.container.UnexpectedResponseException;
import com.hedera.solo.operator.library.container.oci.OCIContentDescriptor;
import com.hedera.solo.operator.library.container.oci.OCIImageIndex;
import com.hedera.solo.operator.library.container.oci.OCIImageManifest;
import java.io.IOException;
import java.util.UUID;
import org.junit.jupiter.api.Test;

@WireMockTest
public class DistributionClientTest {

    @Test
    void testWithImagePullSecret(WireMockRuntimeInfo wmRuntimeInfo) throws IOException, InterruptedException {
        String port = String.valueOf(wmRuntimeInfo.getHttpPort());
        stubs(wmRuntimeInfo);
        assertThat(test(imageCoordinatesHttp(
                        "localhost:{port}/hedera-registry/consensus-node:0.56.0".replace("{port}", port),
                        "{\"auths\":{\"localhost:{port}\":{\"username\":\"user\",\"password\":\"pass\",\"auth\":\"dXNlcjpwYXNz\"}}}"
                                .replace("{port}", port))))
                .isTrue();
    }

    @Test
    void testWithNoAuthority(WireMockRuntimeInfo wmRuntimeInfo) throws IOException, InterruptedException {
        String port = String.valueOf(wmRuntimeInfo.getHttpPort());
        stubFor(get("/v2/").willReturn(ok()));
        stubFor(get("/v2/hedera-registry/consensus-node/manifests/0.56.0")
                .willReturn(unauthorized()
                        .withHeader(
                                "www-authenticate",
                                "Bearer realm=\"http://localhost:{port}/v2/token\"".replace("{port}", port))));
        stubFor(get(urlMatching("/v2/token.*"))
                .willReturn(ok().withHeader("Content-Type", "application/json").withBody("{\"token\":\"jwt\"}")));
        UnexpectedResponseException exception = assertThrows(
                UnexpectedResponseException.class,
                () -> test(imageCoordinatesHttp(
                        "localhost:{port}/hedera-registry/consensus-node:0.56.0".replace("{port}", port))));
        assertThat(exception.response.statusCode()).isEqualTo(HTTP_UNAUTHORIZED);
    }

    @Test
    void testWithExpiration(WireMockRuntimeInfo wmRuntimeInfo) throws IOException, InterruptedException {
        try (DistributionRegistryClient registry = new DistributionRegistryClient()) {
            String image = "localhost:" + wmRuntimeInfo.getHttpPort() + "/hedera-registry/consensus-node:0.56.0";
            ImageCoordinates coordinates = imageCoordinatesHttp(image);
            stubs(wmRuntimeInfo);
            assertThat(registry.fetchManifest(coordinates).response.statusCode())
                    .isEqualTo(200);
            stubs(wmRuntimeInfo);
            assertThat(registry.fetchManifest(coordinates).response.statusCode())
                    .isEqualTo(200);
        }
    }

    boolean test(ImageCoordinates coordinates) throws IOException, InterruptedException {
        try (DistributionRegistryClient registry = new DistributionRegistryClient()) {
            ManifestResponse response = registry.fetchManifest(coordinates);
            OCIContentDescriptor[] layers =
                    switch (response.manifest) {
                        case OCIImageIndex index -> ((OCIImageManifest) registry.fetchManifest(
                                                // just choose the first manifest here
                                                coordinates.digest(index.manifests[0].digest),
                                                index.manifests[0].mediaType)
                                        .manifest)
                                .layers;
                        case OCIImageManifest manifest -> manifest.layers;
                        case null -> throw new RuntimeException();
                    };
            boolean success = true;
            for (OCIContentDescriptor layer : layers) {
                success = success && registry.layerExists(coordinates, layer);
            }
            return success;
        }
    }

    void stubs(WireMockRuntimeInfo wmRuntimeInfo) {
        String token = UUID.randomUUID().toString();
        String authenticate = "Bearer realm=\"http://localhost:" + wmRuntimeInfo.getHttpPort() + "/v2/token\"";
        reset();
        stubFor(get("/v2/").willReturn(unauthorized().withHeader("www-authenticate", authenticate)));
        stubFor(get(urlMatching("/v2/token.*"))
                .willReturn(
                        ok().withHeader("Content-Type", "application/json").withBody("{\"token\":\"" + token + "\"}")));
        stubFor(get(urlMatching("/v2/hedera-registry/consensus-node/manifests/.*"))
                .withHeader("Authorization", not(equalTo("Bearer " + token)))
                .willReturn(unauthorized().withHeader("www-authenticate", authenticate)));
        stubFor(head(urlMatching("/v2/hedera-registry/consensus-node/blobs/.*"))
                .withHeader("Authorization", not(equalTo("Bearer " + token)))
                .willReturn(unauthorized().withHeader("www-authenticate", authenticate)));
        stubFor(
                get("/v2/hedera-registry/consensus-node/manifests/0.56.0")
                        .withHeader("Authorization", equalTo("Bearer " + token))
                        .willReturn(
                                ok().withHeader("Content-Type", "application/vnd.oci.image.index.v1+json")
                                        .withBody(
                                                "{\"manifests\":[{\"mediaType\":\"application/vnd.oci.image.manifest.v1+json\",\"digest\":\"sha256:966c4294346768f7867b3f6a506d02a60ddee563764b81aff2c5073a27a9fead\",\"platform\":{}}]}")));
        stubFor(
                get("/v2/hedera-registry/consensus-node/manifests/sha256:966c4294346768f7867b3f6a506d02a60ddee563764b81aff2c5073a27a9fead")
                        .withHeader("Authorization", equalTo("Bearer " + token))
                        .willReturn(
                                ok().withHeader("Content-Type", "application/vnd.oci.image.manifest.v1+json")
                                        .withBody(
                                                "{\"layers\":[{\"mediaType\":\"application/vnd.oci.image.layer.v1.tar+gzip\",\"digest\":\"hash:1\"}]}")));
        stubFor(head(urlEqualTo("/v2/hedera-registry/consensus-node/blobs/hash:1"))
                .withHeader("Authorization", equalTo("Bearer " + token))
                .willReturn(ok()));
    }

    ImageCoordinates imageCoordinatesHttp(final String image, final String... json) {
        return ImageCoordinates.from("http", image, json);
    }
}
