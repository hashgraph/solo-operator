// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.envoy;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.operator.library.generic.Nullable.orNull;
import static com.hedera.solo.operator.library.model.Labels.soloOperatorDomain;
import static com.hedera.solo.operator.reconciler.envoy.resources.EnvoyUtils.deploymentName;
import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

import com.hedera.solo.operator.model.v1alpha1.EnvoyProxy;
import com.hedera.solo.operator.model.v1alpha1.EnvoyProxySpec;
import io.fabric8.kubernetes.api.model.apps.Deployment;
import io.quarkus.test.junit.QuarkusTest;
import java.time.Duration;
import java.util.Map;
import java.util.UUID;
import java.util.stream.Collectors;
import org.junit.jupiter.api.Test;

/**
 * Test class for verifying the restart functionality of EnvoyProxy resources in a Kubernetes environment. It checks if
 * the Envoy deployment is updated when the configuration files are modified.
 */
@QuarkusTest
public class EnvoyRestartTest extends EnvoyTestAbstract {
    String digest;

    @Test
    void testRestart() throws Exception {
        String digest = digest();

        EnvoyProxySpec spec = new EnvoyProxySpec();
        spec.setEnv(null);
        kubernetesClient
                .resource(edit(new EnvoyProxy())
                        .with(metadata -> metadata.editMetadata()
                                .withName(envoyProxy.getMetadata().getName()))
                        .with(b -> b.withNewSpecLike(spec)
                                .withFiles(envoyProxy.getSpec().getFiles().entrySet().stream()
                                        .collect(Collectors.toMap(
                                                Map.Entry::getKey, e -> e.getValue() + "\n# " + UUID.randomUUID()))))
                        .build())
                .inNamespace(namespace())
                .patch();

        assertThat(digest()).isNotEqualTo(digest);
    }

    /**
     * Retrieves the current digest of the Envoy deployment by checking the annotations in the deployment metadata.
     *
     * @return the digest string
     */
    String digest() {
        await().atMost(Duration.ofSeconds(30)).untilAsserted(() -> {
            Deployment deployment = kubernetesClient
                    .resources(Deployment.class)
                    .inNamespace(namespace())
                    .withName(deploymentName(envoyProxy))
                    .get();
            digest = orNull(() -> deployment
                    .getSpec()
                    .getTemplate()
                    .getMetadata()
                    .getAnnotations()
                    .get(soloOperatorDomain("digest")));
            assertThat(digest).isNotNull();
        });
        return digest;
    }
}
