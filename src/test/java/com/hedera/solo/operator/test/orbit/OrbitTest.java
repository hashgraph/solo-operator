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

import com.hedera.solo.operator.orbit.Orbit;
import com.hedera.solo.operator.orbit.OrbitSpec;
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
        orbit.setSpec(new OrbitSpec());

        client.resource(orbit).create();

        await().atMost(30, TimeUnit.SECONDS).untilAsserted(() -> {
            assertThat(client.resource(orbit).get()).isNotNull();
            assertThat(client.resource(orbit).get().getMetadata().getName()).isEqualTo("test-orbit");
            assertThat(client.resource(orbit).get().getStatus()).isNotNull();
            assertThat(client.resource(orbit).get().getSpec()).isNotNull();
            assertThat(client.resource(orbit).get().getStatus().getObservedGeneration())
                    .isNotNull()
                    .isGreaterThanOrEqualTo(0);
        });
    }
}
