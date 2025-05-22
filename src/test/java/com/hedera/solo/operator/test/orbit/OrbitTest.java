// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.orbit;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.reconciler.common.Metadata;
import com.hedera.solo.operator.reconciler.orbit.resources.ConsensusConfigMap;
import com.hedera.solo.operator.test.fixtures.Resources;
import com.hedera.solo.operator.test.fixtures.annotations.SoloTest;
import com.hedera.solo.operator.test.fixtures.resource.ResourceLoader;
import com.hedera.solo.operator.test.fixtures.snapshot.SnapshotFailureContext;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.Namespace;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.fabric8.kubernetes.client.KubernetesClientBuilder;
import io.javaoperatorsdk.operator.Operator;
import io.quarkus.test.junit.QuarkusTest;
import jakarta.inject.Inject;
import java.io.File;
import java.io.FileInputStream;
import java.time.Duration;
import java.util.Objects;
import java.util.concurrent.TimeUnit;
import org.jboss.logging.Logger;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

/**
 * Unit Test coverage for the Orbit custom resource.
 *
 * <p>NOTE: The tests defined in this suite are only intended for execution in Quarkus Dev Mode.
 */
@QuarkusTest
@SoloTest
class OrbitTest {

    private static final String TEST_ORBIT_NAME = "test-orbit";

    @Inject
    @SuppressWarnings("CdiInjectionPointsInspection")
    protected KubernetesClient client;

    @Inject
    protected Operator operator;

    @Inject
    protected Logger log;

//    @BeforeAll
//    static void initialize() {
//        try {
//            File crdDir = new File("build/kubernetes");
//            for (File file : Objects.requireNonNull(crdDir.listFiles((d, name) -> name.endsWith("v1.yml")))) {
//                Process process = new ProcessBuilder("kubectl", "apply", "-f", file.getAbsolutePath()).inheritIO().start();
//                if (process.waitFor() != 0) {
//                    throw new RuntimeException("Failed to apply CRDs");
//                }
//            }
//        } catch (Exception e) {
//            throw new RuntimeException(e);
//        }
//    }
    @BeforeEach
    void setup() {
        cleanup();
    }

    @AfterEach
    void afterEach(final SnapshotFailureContext snapshot) {
        snapshot.client(client)
                .nonNamespacedResources("operator.solo.hedera.com/v1alpha1")
                .namespaces(TEST_ORBIT_NAME)
                .namespacedResources();
        cleanup();
    }

    void cleanup() {
        Resources.removeByName(client, Orbit.class, TEST_ORBIT_NAME);
        await().atMost(Duration.ofSeconds(30))
                .until(() -> client.namespaces().withName(TEST_ORBIT_NAME).get() == null);
    }

    @Test
    void test() {
        final Orbit orbit = edit(new Orbit())
                .with(metadata -> metadata.editMetadata().withName(TEST_ORBIT_NAME))
                .with(spec -> spec.editSpec().withLifetime("P2DT15H").addToFinalizers("kubernetes"))
                .with(ResourceLoader::withResources)
                .build();

        client.resource(orbit).create();

        await().atMost(30, TimeUnit.SECONDS).untilAsserted(() -> {
            final Orbit realOrbit = client.resource(orbit).get();

            assertThat(realOrbit).isNotNull();
            assertThat(realOrbit.getMetadata().getName()).isEqualTo(TEST_ORBIT_NAME);
            assertThat(realOrbit.getStatus()).isNotNull();
            assertThat(realOrbit.getSpec()).isNotNull();
            assertThat(realOrbit.getSpec().getLifetime()).isEqualTo("P2DT15H");
            assertThat(realOrbit.getSpec().getFinalizers()).containsExactly("kubernetes");
            assertThat(realOrbit.getStatus().getObservedGeneration())
                    .isNotNull()
                    .isNotNegative();

            await().atMost(30, TimeUnit.SECONDS).untilAsserted(() -> {
                final Namespace orbitNs = client.namespaces()
                        .withName(realOrbit.getMetadata().getName())
                        .get();
                assertThat(orbitNs).isNotNull();
                assertThat(orbitNs.getMetadata().getName()).isEqualTo(TEST_ORBIT_NAME);
                assertThat(orbitNs.getMetadata().getLabels())
                        .containsKeys(Metadata.ORBIT_NAME, Metadata.ORBIT_NAMESPACE);
            });

            await().atMost(30, TimeUnit.SECONDS).untilAsserted(() -> {
                final ConfigMap cm = client.configMaps()
                        .inNamespace(realOrbit.getMetadata().getName())
                        .withName(ConsensusConfigMap.NAME)
                        .get();
                assertThat(cm).isNotNull();
                assertThat(cm.getMetadata().getName()).isEqualTo(ConsensusConfigMap.NAME);
                assertThat(cm.getMetadata().getNamespace())
                        .isEqualTo(realOrbit.getMetadata().getName());
                assertThat(cm.getMetadata().getLabels()).containsKeys(Metadata.ORBIT_NAME, Metadata.ORBIT_NAMESPACE);
            });
        });
    }
}
