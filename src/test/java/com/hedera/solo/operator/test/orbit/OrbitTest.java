/*
 * Copyright (C) 2024 Hedera Hashgraph, LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package com.hedera.solo.operator.test.orbit;

import static com.hedera.solo.operator.test.fixtures.snapshot.Snapshot.NO_NAMESPACE;
import static com.hedera.solo.operator.test.fixtures.snapshot.Snapshot.snapshot;
import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.model.v1alpha1.OrbitSpecBuilder;
import com.hedera.solo.operator.reconciler.common.Metadata;
import com.hedera.solo.operator.reconciler.orbit.resources.ConsensusConfigMap;
import com.hedera.solo.operator.test.fixtures.Resources;
import com.hedera.solo.operator.test.fixtures.annotations.AfterTest;
import com.hedera.solo.operator.test.framework.Environment;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.Namespace;
import io.fabric8.kubernetes.api.model.ObjectMetaBuilder;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.javaoperatorsdk.operator.Operator;
import io.quarkus.test.junit.QuarkusTest;
import jakarta.inject.Inject;
import java.io.IOException;
import java.lang.reflect.Method;
import java.time.Duration;
import java.util.concurrent.TimeUnit;
import org.jboss.logging.Logger;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

/**
 * Unit Test coverage for the Orbit custom resource.
 *
 * <p>NOTE: The tests defined in this suite are only intended for execution in Quarkus Dev Mode.
 */
@QuarkusTest
class OrbitTest {

    private static final String TEST_ORBIT_NAME = "test-orbit";

    @Inject
    @SuppressWarnings("CdiInjectionPointsInspection")
    protected KubernetesClient client;

    @Inject
    protected Environment environment;

    @Inject
    protected Operator operator;

    @Inject
    protected Logger log;

    @BeforeEach
    void setup() {
        if (!environment.isDevModeActive()) {
            operator.start();
        }
        cleanup();
    }

    @AfterTest
    void afterTest(Method method, Throwable throwable) throws IOException {
        if (throwable != null) {
            snapshot(method, client, TEST_ORBIT_NAME, NO_NAMESPACE);
        }
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
            });
        });
    }
}
