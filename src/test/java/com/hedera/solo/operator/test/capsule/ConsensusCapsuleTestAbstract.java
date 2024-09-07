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

import static com.hedera.solo.operator.reconciler.common.Constants.CONSENSUS_NODE;
import static com.hedera.solo.operator.reconciler.common.Constants.LABEL_APP;
import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;
import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

import com.google.protobuf.ByteString;
import com.hedera.hashgraph.sdk.AccountId;
import com.hedera.hashgraph.sdk.Client;
import com.hedera.hashgraph.sdk.FileContentsQuery;
import com.hedera.hashgraph.sdk.FileId;
import com.hedera.hashgraph.sdk.NodeAddress;
import com.hedera.hashgraph.sdk.NodeAddressBook;
import com.hedera.hashgraph.sdk.PrivateKey;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsuleBuilder;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsuleStatus;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitialization;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitializationBuilder;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitializationSpec;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitializationStatus;
import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.model.v1alpha1.OrbitBuilder;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.Sidecars;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.SidecarsBuilder;
import com.hedera.solo.operator.reconciler.common.Constants;
import com.hedera.solo.operator.test.Environment;
import com.hedera.solo.operator.test.HederaSdkClientFactory;
import com.hedera.solo.operator.test.fixtures.Resources;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.ObjectMeta;
import io.fabric8.kubernetes.api.model.Pod;
import io.fabric8.kubernetes.api.model.Secret;
import io.fabric8.kubernetes.api.model.Service;
import io.fabric8.kubernetes.api.model.apps.Deployment;
import io.fabric8.kubernetes.client.CustomResource;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.javaoperatorsdk.operator.Operator;
import jakarta.inject.Inject;
import java.time.Duration;
import java.util.*;
import java.util.concurrent.TimeUnit;
import java.util.function.BiFunction;
import org.awaitility.core.ConditionTimeoutException;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

public abstract class ConsensusCapsuleTestAbstract {
    protected final String namespace = getClass().getSimpleName().toLowerCase();
    public static final String RECORD_STREAM_TEMPLATE = "sidecars/record-stream-uploader.yaml";

    @Inject
    @SuppressWarnings("CdiInjectionPointsInspection")
    KubernetesClient kubernetesClient;

    @Inject
    Environment environment;

    @Inject
    Operator operator;

    @BeforeEach
    void setup() {
        if (!environment.isDevModeActive()) {
            operator.start();
        }

        cleanup();

        // create orbit
        createOrbit();
    }

    @AfterEach
    void cleanup() {
        kubernetesClient
                .resources(NetworkInitialization.class)
                .inNamespace(namespace)
                .delete();

        kubernetesClient
                .resources(ConsensusCapsule.class)
                .inNamespace(namespace)
                .delete();

        Resources.removeByName(kubernetesClient, Orbit.class, namespace);

        await().atMost(Duration.ofSeconds(120))
                .until(() -> kubernetesClient.namespaces().withName(namespace).get() == null);
    }

    record NodeInfo(String name, String accountId, int weight, String internal, String external) {}

    ConfigMap prepareClusterConfig(ConfigMap configMap, List<NodeInfo> nodes) {
        // should probably set the chainId in bootstrap.properties
        final Optional<String> cid = configMap
                .getData()
                .get("bootstrap.properties")
                .lines()
                .filter(s -> s.startsWith("contracts.chainId="))
                .findFirst();

        if (cid.isEmpty()) {
            throw new RuntimeException("contacts.chainid property not found");
        }

        String chainId = cid.get().substring(18);

        int nodeId = 0;
        StringBuilder sb = new StringBuilder();
        sb.append("swirld, ").append(chainId).append("\n");
        sb.append("app, HederaNode.jar\n");
        for (NodeInfo node : nodes) {
            sb.append("address, %d, %s, %s, 1, %s, 50111, %s, 50111, 0.0.%d\n"
                    .formatted(nodeId, node.name, node.name, node.internal, node.external, nodeId + 3));
            nodeId++;
        }
        sb.append("nextNodeId, ").append(nodeId);
        return configMap.edit().addToData("config.txt", sb.toString()).build();
    }

    private List<NodeInfo> prepareNodes(List<String> nodeNames) {
        List<NodeInfo> nodes = new ArrayList<>();
        int accStart = Constants.HEDERA_ACCOUNT_START;
        for (String nodeName : nodeNames) {
            nodes.add(new NodeInfo(
                    nodeName,
                    String.format("0.0.%d", accStart++),
                    1,
                    String.format("%s.consensus", nodeName),
                    String.format("%s-svc", nodeName)));
        }

        return nodes;
    }

    private void setupConfigMaps(List<NodeInfo> nodes) {
        // create cluster-config configmap
        kubernetesClient
                .resource(prepareClusterConfig(
                        loadYaml(ConfigMap.class, getClass(), "cluster-config.yaml")
                                .edit()
                                .editMetadata()
                                .withNamespace(namespace)
                                .endMetadata()
                                .build(),
                        nodes))
                .create();

        // create other config-maps
        for (String cmFile : List.of("backup-uploader-config.yaml", "public-keys.yaml")) {
            kubernetesClient
                    .resource(loadYaml(ConfigMap.class, getClass(), cmFile)
                            .edit()
                            .editMetadata()
                            .withNamespace(namespace)
                            .endMetadata()
                            .build())
                    .create();
        }
    }

    private void setupSecrets(List<NodeInfo> nodes) {
        final ArrayList<String> secretFiles = new ArrayList<>(List.of("uploader-secrets.yaml", "mirror-secrets.yaml"));

        for (String sf : secretFiles) {
            kubernetesClient
                    .resource(loadYaml(Secret.class, getClass(), sf)
                            .edit()
                            .editMetadata()
                            .withNamespace(namespace)
                            .endMetadata()
                            .build())
                    .create();
        }

        for (NodeInfo node : nodes) {
            final String nodeKeyFile = String.format("%s-keys.yaml", node.name);
            kubernetesClient
                    .resource(loadYaml(Secret.class, getClass(), nodeKeyFile)
                            .edit()
                            .editMetadata()
                            .withNamespace(namespace)
                            .addToLabels("consensus-capsule", node.name)
                            .endMetadata()
                            .build())
                    .create();
        }
    }

    @Test
    void testSingleNodeDeployment() throws Exception {
        final List<String> nodeNames = List.of("node0");
        final List<NodeInfo> nodes = prepareNodes(nodeNames);
        this.setupConfigMaps(nodes);
        this.setupSecrets(nodes);

        // create consensus capsule
        List<ConsensusCapsule> consensusCapsules = createConsensusCapsule(nodes);

        // create network initialization
        createNetworkInitialization(consensusCapsules);

        // test with client
        checkAddressBook(HederaSdkClientFactory::directContextCreator);
    }

    @Test
    void testMultiNodeDeployment() throws Exception {
        final List<String> nodeNames = List.of("node0", "node1", "node2");
        final List<NodeInfo> nodes = prepareNodes(nodeNames);
        this.setupConfigMaps(nodes);
        this.setupSecrets(nodes);

        // create consensus capsule
        List<ConsensusCapsule> consensusCapsules = createConsensusCapsule(nodes);

        // create network initialization
        createNetworkInitialization(consensusCapsules);

        // test with client
        checkAddressBook(HederaSdkClientFactory::directContextCreator);
    }

    void createOrbit() {
        Orbit orbit = new OrbitBuilder()
                .withNewMetadata()
                .withName(namespace)
                .endMetadata()
                .withNewSpec()
                .endSpec()
                .build();

        kubernetesClient.resource(orbit).create();

        await().atMost(30, TimeUnit.SECONDS)
                .untilAsserted(
                        () -> assertThat(kubernetesClient.resource(orbit).get()).isNotNull());
    }

    protected Sidecars prepareSideCars() {
        return new SidecarsBuilder().build();
    }

    protected List<Pod> fetchConsensusPods() {
        return kubernetesClient
                .resources(Pod.class)
                .inNamespace(namespace)
                .withLabels(Map.of(LABEL_APP, CONSENSUS_NODE))
                .list()
                .getItems();
    }

    List<ConsensusCapsule> createConsensusCapsule(List<NodeInfo> nodes) {
        Sidecars sidecars = this.prepareSideCars();

        List<ConsensusCapsule> list = new ArrayList<>();
        for (NodeInfo node : nodes) {
            final ConsensusCapsule consensusCapsule = new ConsensusCapsuleBuilder()
                    .withNewMetadata()
                    .withName(node.name)
                    .withNamespace(namespace)
                    .endMetadata()
                    .withNewSpec()
                    .withAccountId(node.accountId)
                    .withSidecars(sidecars)
                    .endSpec()
                    .build();
            list.add(kubernetesClient.resource(consensusCapsule).create());
        }

        for (ConsensusCapsule consensusCapsule : list) {
            String name = consensusCapsule.getMetadata().getName();
            await().atMost(30, TimeUnit.SECONDS).untilAsserted(() -> {
                ConsensusCapsule capsule =
                        kubernetesClient.resource(consensusCapsule).get();
                assertThat(capsule).isNotNull();
                Secret secret = kubernetesClient
                        .secrets()
                        .inNamespace(namespace)
                        .withName(name + "-keys")
                        .get();
                assertThat(secret).isNotNull();
                Service service = kubernetesClient
                        .services()
                        .inNamespace(namespace)
                        .withName(name + "-svc")
                        .get();
                assertThat(service).isNotNull();
                assertThat(capsule.getSpec().getEligibleForDeployment()).isFalse();
            });
        }

        return list;
    }

    void createNetworkInitialization(List<ConsensusCapsule> consensusCapsules) {
        final NetworkInitialization networkInitialization = new NetworkInitializationBuilder()
                .withNewMetadata()
                .withName("network-initialization")
                .withNamespace(namespace)
                .endMetadata()
                .withNewSpec()
                .withMode(NetworkInitializationSpec.Mode.FirstInitialization)
                .endSpec()
                .withNewStatus()
                .endStatus()
                .build();
        kubernetesClient.resource(networkInitialization).create();

        await().atMost(90, TimeUnit.SECONDS).untilAsserted(() -> {
            NetworkInitialization netInit =
                    kubernetesClient.resource(networkInitialization).get();
            assertThat(netInit).isNotNull();
            for (ConsensusCapsule consensusCapsule : consensusCapsules) {
                String name = consensusCapsule.getMetadata().getName();
                ConsensusCapsule capsule =
                        kubernetesClient.resource(consensusCapsule).get();
                assertThat(capsule).isNotNull();
                assertThat(capsule.getSpec().getEligibleForDeployment()).isTrue();

                Deployment deployment = kubernetesClient
                        .resources(Deployment.class)
                        .inNamespace(namespace)
                        .withName(name + "-deploy")
                        .get();
                assertThat(deployment).isNotNull();
                assertThat(deployment.getStatus()).isNotNull();
                assertThat(deployment.getStatus().getAvailableReplicas()).isEqualTo(1);

                assertThat(capsule.getStatus().getPhase()).isEqualTo(ConsensusCapsuleStatus.Phase.Running);
                assertThat(netInit.getStatus().getPhase()).isEqualTo(NetworkInitializationStatus.Phase.Succeeded);
            }
        });
    }

    List<NodeAddress> addresses(final Client client) throws Exception {
        final ByteString raw =
                new FileContentsQuery().setFileId(FileId.ADDRESS_BOOK).execute(client);
        assertThat(raw).isNotNull();
        return NodeAddressBook.fromBytes(raw).getNodeAddresses();
    }

    String[] consensusNodeNames() {
        return kubernetesClient.resources(ConsensusCapsule.class).inNamespace(namespace).list().getItems().stream()
                .map(CustomResource::getMetadata)
                .map(ObjectMeta::getName)
                .toArray(String[]::new);
    }

    void checkAddressBook(
            final BiFunction<HederaSdkClientFactory, String, HederaSdkClientFactory.ClientContext> clientContext)
            throws AssertionError, Exception {
        String[] consensusNodeNames = consensusNodeNames();
        HederaSdkClientFactory sdkClientFactory = new HederaSdkClientFactory(kubernetesClient, namespace);
        for (String nodeName : consensusNodeNames) {
            AssertionError assertion = null;
            ConditionTimeoutException timeout = null;
            for (int attempt = 0; attempt < 10; attempt++) {
                assertion = null;
                timeout = null;
                try (HederaSdkClientFactory.ClientContext context = clientContext.apply(sdkClientFactory, nodeName)) {
                    await().atMost(15, TimeUnit.SECONDS).untilAsserted(() -> {
                        Client client = context.client;
                        client.setOperator(
                                AccountId.fromString(HederaSdkClientFactory.DEFAULT_ACCOUNT_ID),
                                PrivateKey.fromString(HederaSdkClientFactory.DEFAULT_PRIVATE_KEY));

                        client.setTransportSecurity(false);
                        assertThat(addresses(client).size()).isEqualTo(consensusNodeNames.length);

                        client.setTransportSecurity(true).setVerifyCertificates(true);
                        assertThat(addresses(client).size()).isEqualTo(consensusNodeNames.length);
                    });
                } catch (AssertionError e) {
                    assertion = e;
                } catch (ConditionTimeoutException e) {
                    timeout = e;
                }
                if (assertion == null && timeout == null) {
                    break;
                }
            }
            if (assertion != null) {
                throw assertion;
            }
            if (timeout != null) {
                throw timeout;
            }
        }
    }
}
