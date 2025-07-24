// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.fixtures.microcks;

import io.fabric8.kubernetes.client.LocalPortForward;
import java.io.IOException;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.Duration;
import java.util.Arrays;
import java.util.List;
import java.util.function.Function;

/**
 * EchoService provides methods to interact with a gRPC-Web Echo service. It includes functionality to send echo
 * requests and retrieve the necessary resources for the service.
 */
public class EchoService {

    /**
     * Returns a list of resources required for the Echo service. The resources include the Echo service's protobuf
     * definition and its JSON representation.
     *
     * @return A list containing the paths to the Echo service's proto and JSON files
     */
    public static List<List<String>> resources() {
        return List.of(List.of("io/github/microcks/echo.proto", "io/github/microcks/echo.json"));
    }

    /**
     * Sends an echo request to the Echo service running on the specified local port forward.
     *
     * @param portForward The local port forward to connect to the Echo service
     * @param type The content type of the request (e.g., "grpc-web", "grpc-web+proto", "grpc-web-text")
     * @param value The integer value to be echoed back by the service
     * @param function A function that converts the request payload into a suitable body publisher
     * @return true if the response matches the expected echoed value, false otherwise
     * @throws IOException If an I/O error occurs during the request
     * @throws InterruptedException If the request is interrupted
     */
    public static boolean echo(
            final LocalPortForward portForward,
            final String type,
            final int value,
            final Function<byte[], HttpRequest.BodyPublisher> function)
            throws IOException, InterruptedException {
        HttpRequest request = HttpRequest.newBuilder()
                .uri(URI.create("http://localhost:" + portForward.getLocalPort() + "/echo.v1.EchoService/Echo"))
                .header("Content-Type", "application/" + type)
                .header("X-Grpc-Web", "1")
                .timeout(Duration.ofSeconds(10))
                .POST(function.apply(request(value)))
                .build();

        try (HttpClient httpClient = HttpClient.newHttpClient()) {
            HttpResponse<byte[]> response = httpClient.send(request, HttpResponse.BodyHandlers.ofByteArray());
            return response.statusCode() == 200 && Arrays.equals(response.body(), response(value));
        }
    }

    // spotless:off
    /**
     * Builds a minimal gRPC-Web request payload.
     *
     * The format follows the gRPC-Web binary framing:
     * - 1 byte: compressed flag (0x00 = not compressed)
     * - 4 bytes: message length (big-endian)
     * - N bytes: the protobuf message payload
     *
     * This constructs a request where the message is a 5-byte payload:
     * - First byte: 0x0d (some fixed identifier or type)
     * - Next 4 bytes: the given `value` integer encoded in big-endian format
     *
     * @param value A 32-bit integer to include in the message payload
     * @return A byte array representing the framed gRPC-Web request
     */
    public static byte[] request(final int value) {
        return new byte[] {
                0x00,                               // Compressed-flag: 0 (not compressed)
                0x00, 0x00, 0x00, 0x05,             // Message length: 5 bytes (0x00000005)
                0x0d,                               // Payload[0]: fixed byte (e.g., message type or method id)
                (byte) (value >> 24 & 0xff),        // Payload[1]: most significant byte of value
                (byte) (value >> 16 & 0xff),        // Payload[2]
                (byte) (value >> 8 & 0xff),         // Payload[3]
                (byte) (value & 0xff)               // Payload[4]: least significant byte of value
        };
    }
    // spotless:on

    // spotless:off
    /**
     * Builds a gRPC-Web response payload.
     *
     * This method returns a byte array that simulates a complete gRPC-Web response,
     * including a message and the gRPC status trailer.
     *
     * Format:
     * 1. Message frame (binary format):
     *    - 1 byte: compressed-flag (0 = not compressed)
     *    - 4 bytes: message length (big-endian) = 5
     *    - 5 bytes: message payload
     *        - 1 byte: fixed identifier (0x0d)
     *        - 4 bytes: integer value in big-endian
     *
     * 2. Trailer frame (starts with 0x80):
     *    - 1 byte: trailer flag (0x80 indicates trailer frame)
     *    - 4 bytes: trailer length (big-endian) = 15
     *    - 15 bytes: ASCII string "grpc-status:0\r\n"
     *
     * @param value Integer to be encoded in the response payload
     * @return Byte array representing the gRPC-Web response
     */
    public static byte[] response(final int value) {
        return new byte[] {
                // --- gRPC message frame ---
                0x00,                               // Compressed-flag: 0 (not compressed)
                0x00, 0x00, 0x00, 0x05,             // Message length: 5 bytes (0x00000005)
                0x0d,                               // Payload[0]: fixed byte (e.g., message type or response ID)
                (byte) (value >> 24 & 0xff),        // Payload[1]: most significant byte of value
                (byte) (value >> 16 & 0xff),        // Payload[2]
                (byte) (value >> 8 & 0xff),         // Payload[3]
                (byte) (value & 0xff),              // Payload[4]: least significant byte of value

                // --- gRPC trailers frame ---
                (byte) 0x80,                        // Trailer flag (MSB set means trailers follow)
                0x00, 0x00, 0x00, 0x0f,             // Length of trailer data: 15 bytes
                'g', 'r', 'p', 'c', '-',            // Start of ASCII trailer string
                's', 't', 'a', 't', 'u', 's',       // "grpc-status"
                ':', '0',                           // Status value "0" = OK
                '\r', '\n'                          // Trailer terminator (CRLF)
        };
    }
    // spotless:on
}
