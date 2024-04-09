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

import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.model.v1alpha1.OrbitSpecBuilder;
import com.hedera.solo.operator.reconciler.common.Metadata;
import com.hedera.solo.operator.reconciler.orbit.resources.ConsensusConfigMap;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.Namespace;
import io.fabric8.kubernetes.api.model.ObjectMetaBuilder;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.javaoperatorsdk.operator.Operator;
import io.quarkus.test.junit.QuarkusTest;
import jakarta.inject.Inject;
import java.util.concurrent.TimeUnit;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

@QuarkusTest
class OrbitTest {
    @Inject
    @SuppressWarnings("CdiInjectionPointsInspection")
    protected KubernetesClient client;

    @Inject
    protected Operator operator;

    @BeforeEach
    void setup() {
        operator.start();
    }

    @AfterEach
    void cleanup() {
        operator.stop();
    }

    @Test
    void test() {
        final Orbit orbit = new Orbit();
        orbit.setMetadata(new ObjectMetaBuilder().withName("test-orbit").build());
        orbit.setSpec(new OrbitSpecBuilder()
                .withLifetime("P2DT15H")
                .addToFinalizers("kubernetes")
                .build());

        client.resource(orbit).create();

        await().atMost(30, TimeUnit.SECONDS).untilAsserted(() -> {
            final Orbit realOrbit = client.resource(orbit).get();

            assertThat(realOrbit).isNotNull();
            assertThat(realOrbit.getMetadata().getName()).isEqualTo("test-orbit");
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
                assertThat(orbitNs.getMetadata().getName()).isEqualTo("test-orbit");
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
