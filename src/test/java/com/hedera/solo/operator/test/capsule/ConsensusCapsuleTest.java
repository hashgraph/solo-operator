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

package com.hedera.solo.operator.test.capsule;

import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;
import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsuleBuilder;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsuleStatus;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitialization;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitializationBuilder;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitializationSpec;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitializationStatus;
import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.model.v1alpha1.OrbitBuilder;
import com.hedera.solo.operator.test.Environment;
import com.hedera.solo.operator.test.fixtures.Resources;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.Secret;
import io.fabric8.kubernetes.api.model.Service;
import io.fabric8.kubernetes.api.model.apps.Deployment;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.javaoperatorsdk.operator.Operator;
import io.quarkus.test.junit.QuarkusTest;
import jakarta.inject.Inject;
import java.lang.invoke.MethodHandles;
import java.time.Duration;
import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.TimeUnit;
import java.util.function.Function;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Disabled;
import org.junit.jupiter.api.Test;

@Disabled
@QuarkusTest
public class ConsensusCapsuleTest {
    private static final String NAMESPACE =
            MethodHandles.lookup().lookupClass().getSimpleName().toLowerCase();
    private static final String TEST_ORBIT_NAME = NAMESPACE;

    @Inject
    @SuppressWarnings("CdiInjectionPointsInspection")
    protected KubernetesClient client;

    @Inject
    protected Environment environment;

    @Inject
    protected Operator operator;

    @BeforeEach
    void setup() {
        if (!environment.isDevModeActive()) {
            operator.start();
        }
        cleanup();
    }

    @AfterEach
    void cleanup() {
        client.resources(NetworkInitialization.class).inNamespace(NAMESPACE).delete();
        client.resources(ConsensusCapsule.class).inNamespace(NAMESPACE).delete();
        Resources.removeByName(client, Orbit.class, TEST_ORBIT_NAME);
        await().atMost(Duration.ofSeconds(30))
                .until(() -> client.namespaces().withName(TEST_ORBIT_NAME).get() == null);
    }

    record Node(String name, int weight, String internal, String external) {}

    ConfigMap config(ConfigMap configMap, Node... nodes) {
        // should probably set the chainId in bootstrap.properties
        String chainId = configMap
                .getData()
                .get("bootstrap.properties")
                .lines()
                .filter(s -> s.startsWith("contracts.chainId="))
                .findFirst()
                .get()
                .substring(18);
        int nodeId = 0;
        StringBuilder sb = new StringBuilder();
        sb.append("swirld, ").append(chainId).append("\n");
        sb.append("app, HederaNode.jar\n");
        for (Node node : nodes) {
            sb.append("address, %d, %s, %s, 1, %s, 50111, %s, 50111, 0.0.%d\n"
                    .formatted(nodeId, node.name, node.name, node.internal, node.external, nodeId + 3));
            nodeId++;
        }
        sb.append("nextNodeId, ").append(nodeId);
        return configMap.edit().addToData("config.txt", sb.toString()).build();
    }

    @Test
    void testSingleNodeDeployment() {
        // create orbit
        createOrbit();

        client.resource(loadYaml(ConfigMap.class, getClass(), "public-keys.yaml")
                        .edit()
                        .editMetadata()
                        .withNamespace(NAMESPACE)
                        .endMetadata()
                        .build())
                .create();
        client.resource(loadYaml(ConfigMap.class, getClass(), "cluster-config.yaml")
                        .edit()
                        .editMetadata()
                        .withNamespace(NAMESPACE)
                        .endMetadata()
                        .build())
                .create();

        // create consensus capsule
        List<ConsensusCapsule> consensusCapsules = createConsensusCapsule("node0");

        // create network initialization
        createNetworkInitialization(consensusCapsules);
    }

    @Test
    void testMultiNodeDeployment() {
        // create orbit
        createOrbit();

        // populate config and keys
        Function<String, String> externalName = s -> s + "-svc." + NAMESPACE + "." + "svc.cluster.local";
        client.resource(config(
                        loadYaml(ConfigMap.class, getClass(), "cluster-config.yaml")
                                .edit()
                                .editMetadata()
                                .withNamespace(NAMESPACE)
                                .endMetadata()
                                .build(),
                        new Node("node0", 1, "node0", externalName.apply("node0")),
                        new Node("node1", 1, "node1", externalName.apply("node1")),
                        new Node("node2", 1, "node2", externalName.apply("node2"))))
                .create();
        client.resource(loadYaml(ConfigMap.class, getClass(), "public-keys.yaml")
                        .edit()
                        .editMetadata()
                        .withNamespace(NAMESPACE)
                        .endMetadata()
                        .build())
                .create();

        // create consensus capsule
        List<ConsensusCapsule> nodes = createConsensusCapsule("node0", "node1", "node2");

        client.resource(loadYaml(Secret.class, getClass(), "node0-keys.yaml"))
                .inNamespace(NAMESPACE)
                .patch();
        client.resource(loadYaml(Secret.class, getClass(), "node1-keys.yaml"))
                .inNamespace(NAMESPACE)
                .patch();
        client.resource(loadYaml(Secret.class, getClass(), "node2-keys.yaml"))
                .inNamespace(NAMESPACE)
                .patch();

        // create network initialization
        createNetworkInitialization(nodes);
    }

    Orbit createOrbit() {
        Orbit orbit = new OrbitBuilder()
                .withNewMetadata()
                .withName(TEST_ORBIT_NAME)
                .endMetadata()
                .withNewSpec()
                .endSpec()
                .build();
        client.resource(orbit).create();

        await().atMost(30, TimeUnit.SECONDS).untilAsserted(() -> {
            assertThat(client.resource(orbit).get()).isNotNull();
        });

        return orbit;
    }

    List<ConsensusCapsule> createConsensusCapsule(String... names) {
        List<ConsensusCapsule> list = new ArrayList<>();
        for (String name : names) {
            final ConsensusCapsule consensusCapsule = new ConsensusCapsuleBuilder()
                    .withNewMetadata()
                    .withName(name)
                    .withNamespace(NAMESPACE)
                    .endMetadata()
                    .withNewSpec()
                    .endSpec()
                    .build();
            list.add(client.resource(consensusCapsule).create());
        }
        for (ConsensusCapsule consensusCapsule : list) {
            String name = consensusCapsule.getMetadata().getName();
            await().atMost(30, TimeUnit.SECONDS).untilAsserted(() -> {
                ConsensusCapsule capsule = client.resource(consensusCapsule).get();
                assertThat(capsule).isNotNull();
                Secret secret = client.secrets()
                        .inNamespace(NAMESPACE)
                        .withName(name + "-keys")
                        .get();
                assertThat(secret).isNotNull();
                Service service = client.services()
                        .inNamespace(NAMESPACE)
                        .withName(name + "-svc")
                        .get();
                assertThat(service).isNotNull();
                assertThat(capsule.getSpec().getEligibleForDeployment()).isFalse();
            });
        }
        return list;
    }

    NetworkInitialization createNetworkInitialization(List<ConsensusCapsule> consensusCapsules) {
        final NetworkInitialization networkInitialization = new NetworkInitializationBuilder()
                .withNewMetadata()
                .withName("network-initialization")
                .withNamespace(NAMESPACE)
                .endMetadata()
                .withNewSpec()
                .withMode(NetworkInitializationSpec.Mode.FirstInitialization)
                .endSpec()
                .withNewStatus()
                .endStatus()
                .build();
        client.resource(networkInitialization).create();

        await().atMost(99990, TimeUnit.SECONDS).untilAsserted(() -> {
            NetworkInitialization netInit =
                    client.resource(networkInitialization).get();
            assertThat(netInit).isNotNull();
            for (ConsensusCapsule consensusCapsule : consensusCapsules) {
                String name = consensusCapsule.getMetadata().getName();
                ConsensusCapsule capsule = client.resource(consensusCapsule).get();
                assertThat(capsule).isNotNull();
                assertThat(capsule.getSpec().getEligibleForDeployment()).isTrue();

                Deployment deployment = client.resources(Deployment.class)
                        .inNamespace(NAMESPACE)
                        .withName(name + "-deploy")
                        .get();
                assertThat(deployment).isNotNull();
                assertThat(deployment.getStatus()).isNotNull();
                assertThat(deployment.getStatus().getAvailableReplicas()).isEqualTo(1);

                assertThat(capsule.getStatus().getPhase()).isEqualTo(ConsensusCapsuleStatus.Phase.Running);
                assertThat(netInit.getStatus().getPhase()).isEqualTo(NetworkInitializationStatus.Phase.Succeeded);
            }
        });
        return networkInitialization;
    }
}
