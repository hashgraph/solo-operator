// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.crdpostprocessor;

import static org.junit.jupiter.api.Assertions.assertTrue;

import io.fabric8.kubernetes.api.model.apiextensions.v1.CustomResourceDefinition;
import io.fabric8.kubernetes.client.utils.ApiVersionUtil;
import io.fabric8.kubernetes.client.utils.Serialization;
import java.io.IOException;
import java.net.URISyntaxException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.Objects;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.ValueSource;

/**
 * Tests the SharedCrdPostProcessor class, which processes CustomResourceDefinition (CRD) YAML files. This class is used
 * to ensure that CRD files are correctly processed and formatted.
 */
public class SharedCrdPostProcessorTest {
    /**
     * Tests the SharedCrdPostProcessor with various CRD YAML files. It reads input CRD files, processes them, and
     * compares the output with expected results.
     *
     * @param name The name of the CRD YAML file to test.
     * @throws URISyntaxException if the URI syntax is incorrect.
     * @throws IOException if an I/O error occurs while reading or writing files.
     */
    @ParameterizedTest
    @ValueSource(strings = {"crontabs.stable.example.com-v1.yaml", "examples.stable.example.com-v1.yaml"})
    public void test(String name) throws URISyntaxException, IOException {
        String output = read("test/output/" + name);
        String input = read("test/input/" + name);
        Path extra = path("test/extra");
        CustomResourceDefinition crd = Serialization.unmarshal(input, CustomResourceDefinition.class);
        SharedCrdPostProcessor processor = new SharedCrdPostProcessor(extra);
        CustomResourceDefinition result =
                (CustomResourceDefinition) processor.process(crd, ApiVersionUtil.trimVersion(crd.getApiVersion()));
        assertTrue(output.endsWith(Serialization.asYaml(result)));
    }

    /**
     * Reads a resource file and returns its content as a string.
     *
     * @param path The path to the resource file.
     * @return The content of the resource file as a string
     * @throws IOException if an I/O error occurs while reading the file.
     */
    String read(String path) throws IOException {
        return new String(
                Objects.requireNonNull(getClass().getResourceAsStream(path)).readAllBytes(), StandardCharsets.UTF_8);
    }

    /**
     * Converts a resource path to a {@link Path} object.
     *
     * @param resource The resource path as a string.
     * @return A {@link Path} object representing the resource path
     * @throws URISyntaxException if the URI syntax is incorrect.
     */
    Path path(String resource) throws URISyntaxException {
        return Paths.get(
                Objects.requireNonNull(getClass().getResource(resource)).toURI());
    }
}
