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

public class SharedCrdPostProcessorTest {
    @ParameterizedTest
    @ValueSource(strings = {"crontabs.stable.example.com-v1.yaml", "examples.stable.example.com-v1.yaml"})
    public void test(String name) throws URISyntaxException, IOException {
        String expect = read("test/expect/" + name);
        String input = read("test/input/" + name);
        Path augment = path("test/augment");
        CustomResourceDefinition crd = Serialization.unmarshal(input, CustomResourceDefinition.class);
        SharedCrdPostProcessor processor = new SharedCrdPostProcessor(augment);
        CustomResourceDefinition result =
                (CustomResourceDefinition) processor.process(crd, ApiVersionUtil.trimVersion(crd.getApiVersion()));
        assertTrue(expect.endsWith(Serialization.asYaml(result)));
    }

    String read(String path) throws IOException {
        return new String(
                Objects.requireNonNull(getClass().getResourceAsStream(path)).readAllBytes(), StandardCharsets.UTF_8);
    }

    Path path(String resource) throws URISyntaxException {
        return Paths.get(
                Objects.requireNonNull(getClass().getResource(resource)).toURI());
    }
}
