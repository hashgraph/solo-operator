// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.orbit;

import static com.hedera.solo.operator.library.resource.SystemResource.getSystemResourceAsString;
import static com.hedera.solo.operator.test.fixtures.Environment.isTestUsingGradleOrIntelliJ;
import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.model.v1alpha1.OrbitBuilder;
import com.hedera.solo.operator.model.v1alpha1.OrbitSpecBuilder;
import com.hedera.solo.operator.reconciler.common.Metadata;
import com.hedera.solo.operator.reconciler.orbit.resources.ConsensusConfigMap;
import com.hedera.solo.operator.test.fixtures.Resources;
import com.hedera.solo.operator.test.fixtures.annotations.SoloTest;
import com.hedera.solo.operator.test.fixtures.snapshot.SnapshotFailureContext;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.Namespace;
import io.fabric8.kubernetes.api.model.ObjectMetaBuilder;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.javaoperatorsdk.operator.Operator;
import io.quarkus.test.junit.QuarkusTest;
import jakarta.inject.Inject;
import java.time.Duration;
import java.util.concurrent.TimeUnit;
import org.jboss.logging.Logger;
import org.junit.jupiter.api.AfterEach;
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

    @BeforeEach
    void setup() {
        if (isTestUsingGradleOrIntelliJ()) {
            operator.start();
        }
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
        final Orbit orbit = new Orbit();
        orbit.setMetadata(new ObjectMetaBuilder().withName(TEST_ORBIT_NAME).build());
        orbit.setSpec(new OrbitSpecBuilder()
                .withLifetime("P2DT15H")
                .addToFinalizers("kubernetes")
                .build());

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
                // verify default configuration files match bundled resources
                assertThat(cm.getData().get("api-permission.properties"))
                        .isEqualTo(getSystemResourceAsString("import/hedera/mainnet/api-permission.properties"));
                assertThat(cm.getData().get("application.properties"))
                        .isEqualTo(getSystemResourceAsString("import/hedera/mainnet/application.properties"));
                assertThat(cm.getData().get("settings.txt"))
                        .isEqualTo(getSystemResourceAsString("import/hedera/mainnet/settings.txt"));
            });
        });

        // verify default configurations can be overridden
        client.resource(orbit).edit(o -> new OrbitBuilder(o)
                .editSpec()
                .editConsensus()
                .addToApplicationConfiguration("config1", "value1")
                .addToApplicationConfiguration("config2", "value2")
                .addToNetworkSettings("setting1", "value1")
                .addToNetworkSettings("setting2", "value2")
                .addToPermissionConfiguration("permission1", "value1")
                .addToPermissionConfiguration("permission2", "value2")
                .endConsensus()
                .endSpec()
                .build());

        await().atMost(30, TimeUnit.SECONDS).untilAsserted(() -> {
            final Orbit realOrbit = client.resource(orbit).get();
            final ConfigMap cm = client.configMaps()
                    .inNamespace(realOrbit.getMetadata().getName())
                    .withName(ConsensusConfigMap.NAME)
                    .get();
            assertThat(cm).isNotNull();
            assertThat(cm.getMetadata().getName()).isEqualTo(ConsensusConfigMap.NAME);
            assertThat(cm.getMetadata().getNamespace())
                    .isEqualTo(realOrbit.getMetadata().getName());
            assertThat(cm.getMetadata().getLabels()).containsKeys(Metadata.ORBIT_NAME, Metadata.ORBIT_NAMESPACE);
            assertThat(cm.getData().get("application.properties")).isEqualTo("config1=value1\nconfig2=value2\n");
            assertThat(cm.getData().get("settings.txt")).isEqualTo("setting1,value1\nsetting2,value2\n");
            assertThat(cm.getData().get("api-permission.properties"))
                    .isEqualTo("permission1=value1\npermission2=value2\n");
        });
    }
}
