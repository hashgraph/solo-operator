// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.envoy;

import static com.hedera.solo.operator.reconciler.envoy.resources.EnvoyUtils.serviceName;
import static com.hedera.solo.operator.test.fixtures.microcks.EchoService.echo;
import static org.assertj.core.api.Assertions.assertThat;

import io.fabric8.kubernetes.client.LocalPortForward;
import io.quarkus.test.junit.QuarkusTest;
import java.net.http.HttpRequest;
import java.time.Duration;
import java.util.Base64;
import java.util.function.Function;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

/**
 * This test class verifies the functionality of gRPC-Web requests through an Envoy proxy. It uses a local port forward
 * to connect to the Echo service and sends various types of gRPC-Web requests.
 */
@QuarkusTest
public class EnvoyGrpcWebTest extends EnvoyTestAbstract {

    /**
     * Sets up the test environment by ensuring the Microcks service is ready. This is necessary to handle gRPC-Web
     * requests properly.
     */
    @BeforeEach
    void setUp() {
        super.setUp();
        microcksService.waitUntilReady(Duration.ofSeconds(60));
    }

    @Test
    void test() throws Exception {
        try (LocalPortForward portForward = kubernetesClient
                .services()
                .inNamespace(namespace())
                .withName(serviceName(envoyProxy))
                .portForward(envoyProxy.getSpec().getProxyPort())) {

            test(portForward, "grpc-web", HttpRequest.BodyPublishers::ofByteArray);
            test(portForward, "grpc-web+proto", HttpRequest.BodyPublishers::ofByteArray);
            test(
                    portForward,
                    "grpc-web-text",
                    bytes -> HttpRequest.BodyPublishers.ofByteArray(
                            Base64.getEncoder().encode(bytes)));
        }
    }

    /**
     * Sends an echo request to the Echo service through the Envoy proxy and verifies the response.
     *
     * @param portForward The local port forward to connect to the Echo service
     * @param type The content type of the request (e.g., "grpc-web", "grpc-web+proto", "grpc-web-text")
     * @param function A function that converts the request payload into a suitable body publisher
     * @throws Exception If an error occurs during the request
     */
    void test(
            final LocalPortForward portForward,
            final String type,
            final Function<byte[], HttpRequest.BodyPublisher> function)
            throws Exception {
        assertThat(echo(portForward, type, beanFactory.nonNull(Integer.class), function))
                .isTrue();
    }
}
