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

package com.hedera.solo.test.networkinitialization;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.operator.model.v1alpha1.NetworkInitializationSpec.Mode.FirstInitialization;
import static com.hedera.solo.operator.reconciler.common.Metadata.OPERATOR_PREFIX;
import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitialization;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitializationStatus;
import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.test.fixtures.annotations.SoloTest;
import com.hedera.solo.operator.test.fixtures.snapshot.SnapshotFailureContext;
import com.hedera.solo.test.framework.AbstractOperatorTest;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.javaoperatorsdk.operator.api.config.ControllerConfigurationOverrider;
import io.javaoperatorsdk.operator.junit.LocallyRunOperatorExtension;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.concurrent.TimeUnit;
import java.util.stream.Collectors;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

@SoloTest
public class NetworkInitializationTest extends AbstractOperatorTest {
    @Override
    protected LocallyRunOperatorExtension.Builder configure(final KubernetesClient client) {
        return builder(client)
                .withReconciler(
                        new NetworkInitializationTestReconciler(),
                        ControllerConfigurationOverrider::watchingAllNamespaces)
                .withReconciler(new ConsensusCapsuleTestReconciler())
                .withReconciler(new NoOpOrbitReconciler());
    }

    @BeforeEach
    void setup() {
        createOrbit();
    }

    @AfterEach
    void teardown(final SnapshotFailureContext snapshot) {
        snapshot.client(operator().getKubernetesClient())
                .namespaces(operator().getNamespace())
                .namespacedResources();
        operator()
                .getKubernetesClient()
                .resources(Orbit.class)
                .withName(operator().getNamespace())
                .delete();
    }

    @Test
    void OneNetworkInitializationAtATime() {
        int total = 20;
        List<String> capsuleNames = List.of("alpha", "beta", "gamma");

        capsuleNames.forEach(this::createCapsule);

        for (int i = 0; i < total; ++i) {
            String name = "init-" + i;
            createNetworkInitialization(name, false);
            await().atMost(30, TimeUnit.SECONDS).untilAsserted(() -> {
                verifyNetworkInitializationSucceeded(name, true);
            });
        }
        verifyClusterConfig(capsuleNames);
    }

    @Test
    void ManyNetworkInitializationsAtOnceWithDeletions() {
        int total = 20;
        int noUpdatePeriod = 7;
        List<String> capsuleNames = List.of("alpha", "beta", "gamma");

        capsuleNames.forEach(this::createCapsule);

        for (int i = 0; i < total; ++i) {
            createNetworkInitialization("init-" + i, 0 == i % noUpdatePeriod);
        }

        for (int i = 0; i < total; i += noUpdatePeriod) {
            deleteNetworkInitializationStuckInPending("init-" + i);
        }

        await().atMost(30, TimeUnit.SECONDS).untilAsserted(() -> {
            for (int i = 0; i < total; ++i) {
                verifyNetworkInitializationSucceeded("init-" + i, 0 != i % noUpdatePeriod);
            }
        });

        verifyClusterConfig(capsuleNames);
    }

    void createNetworkInitialization(final String name, final boolean noUpdate) {
        operator()
                .create(edit(new NetworkInitialization())
                        .with(metadata -> metadata.editMetadata()
                                .withName(name)
                                .addToLabels("no-update", String.valueOf(noUpdate)))
                        .with(spec -> spec.editSpec().withMode(FirstInitialization))
                        .build());
    }

    void verifyClusterConfig(final List<String> capsuleNames) {
        ConfigMap cm = operator().get(ConfigMap.class, "cluster-config");
        assertThat(cm).isNotNull();
        String configTxt = cm.getData().get("config.txt");
        assertThat(configTxt).isNotNull();
        List<String> lines =
                configTxt.lines().filter(line -> line.startsWith("address")).toList();
        assertThat(lines).size().isEqualTo(3);
        Map<String, Integer> nodes = lines.stream()
                .map(line -> line.split("\\s*,\\s*"))
                .collect(Collectors.toMap(a -> a[2], a -> Integer.valueOf(a[1])));
        assertThat(nodes.keySet()).isEqualTo(new HashSet<>(capsuleNames));
        assertThat(nodes.values().stream().distinct().count()).isEqualTo(capsuleNames.size());
        assertThat(nodes.values().stream().min(Integer::compareTo)).contains(0);
        assertThat(nodes.values().stream().max(Integer::compareTo)).contains(capsuleNames.size() - 1);
    }

    void deleteNetworkInitializationStuckInPending(final String name) {
        await().atMost(30, TimeUnit.SECONDS).untilAsserted(() -> {
            NetworkInitialization ni = operator().get(NetworkInitialization.class, name);
            assertThat(ni).isNotNull();
            assertThat(ni.getStatus()).isNotNull();
            assertThat(ni.getStatus().getPhase()).isEqualTo(NetworkInitializationStatus.Phase.Pending);
        });
        sleep(1000);
        assertThat(operator().get(NetworkInitialization.class, name).getStatus().getPhase())
                .isEqualTo(NetworkInitializationStatus.Phase.Pending);
        operator().delete(operator().get(NetworkInitialization.class, name));
    }

    void verifyNetworkInitializationSucceeded(final String name, final boolean exists) {
        NetworkInitialization ni = operator().get(NetworkInitialization.class, name);
        if (!exists) {
            assertThat(ni).isNull();
        } else {
            assertThat(ni).isNotNull();
            assertThat(ni.getStatus()).isNotNull();
            assertThat(ni.getStatus().getPhase()).isEqualTo(NetworkInitializationStatus.Phase.Succeeded);
        }
    }

    void createOrbit() {
        operator()
                .getKubernetesClient()
                .resource(edit(new Orbit())
                        .with(metadata ->
                                metadata.editMetadata().withName(operator().getNamespace()))
                        .build())
                .create();

        operator()
                .getKubernetesClient()
                .namespaces()
                .withName(operator().getNamespace())
                .edit(namespace -> edit(namespace)
                        .with(metadata -> metadata.editMetadata()
                                .addToLabels(
                                        OPERATOR_PREFIX + "orbit-name",
                                        operator().getNamespace()))
                        .build());
    }

    void createCapsule(final String name) {
        operator()
                .create(edit(new ConsensusCapsule())
                        .with(metadata -> metadata.editMetadata().withName(name))
                        .with(spec -> spec.editSpec().withOrbit(operator().getNamespace()))
                        .build());
    }

    void sleep(final long millis) {
        try {
            Thread.sleep(millis);
        } catch (InterruptedException e) {
            throw new RuntimeException(e);
        }
    }
}
