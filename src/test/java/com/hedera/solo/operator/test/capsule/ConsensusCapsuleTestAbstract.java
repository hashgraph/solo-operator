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

import static com.hedera.solo.operator.test.fixtures.snapshot.Snapshot.NO_NAMESPACE;
import static com.hedera.solo.operator.test.fixtures.snapshot.Snapshot.snapshot;
import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;
import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

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
import com.hedera.solo.operator.test.fixtures.Resources;
import com.hedera.solo.operator.test.fixtures.annotations.AfterTest;
import com.hedera.solo.operator.test.framework.Environment;
import com.hedera.solo.operator.test.framework.HederaSdkClientFactory;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.ObjectMeta;
import io.fabric8.kubernetes.api.model.Secret;
import io.fabric8.kubernetes.api.model.Service;
import io.fabric8.kubernetes.api.model.apps.Deployment;
import io.fabric8.kubernetes.client.CustomResource;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.javaoperatorsdk.operator.Operator;
import jakarta.inject.Inject;
import java.lang.invoke.MethodHandles;
import java.lang.reflect.Method;
import java.net.BindException;
import java.time.Duration;
import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.TimeUnit;
import java.util.function.BiFunction;
import org.awaitility.core.ConditionTimeoutException;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

public abstract class ConsensusCapsuleTestAbstract {
    protected String namespace;

    @Inject
    @SuppressWarnings("CdiInjectionPointsInspection")
    protected KubernetesClient kubernetesClient;

    @Inject
    protected Environment environment;

    @Inject
    protected Operator operator;

    public ConsensusCapsuleTestAbstract() {
        setNamespace(MethodHandles.lookup().lookupClass());
    }

    protected void setNamespace(final Class<?> aClass) {
        namespace = aClass.getSimpleName().toLowerCase();
    }

    @BeforeEach
    void setup() {
        if (!environment.isDevModeActive()) {
            operator.start();
        }
        cleanup();
    }

    @AfterTest
    void afterTest(Method method, Throwable throwable) throws Exception {
        if (throwable != null) {
            snapshot(method, kubernetesClient, namespace, NO_NAMESPACE)
                    .exec(
                            "current-address-book",
                            "app=consensus-node",
                            "consensus-node",
                            """
                                    set -x
                                    find services-hedera/HapiApp2.0/data -type f -name currentAddressBook.txt
                                    find services-hedera/HapiApp2.0/data -type f -name currentAddressBook.txt | \\
                                        xargs ls -1t | head -n 1 | xargs cat
                                    """)
                    // add support for reading files.
                    .exec(
                            "swirlds-log",
                            "app=consensus-node",
                            "consensus-node",
                            "set -x; cat services-hedera/HapiApp2.0/output/swirlds.log");
        }
        cleanup();
    }

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
        await().atMost(Duration.ofSeconds(150))
                .until(() -> kubernetesClient.namespaces().withName(namespace).get() == null);
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
    void testSingleNodeDeployment() throws Exception {
        // create orbit
        createOrbit();

        kubernetesClient
                .resource(loadYaml(ConfigMap.class, getClass(), "public-keys.yaml")
                        .edit()
                        .editMetadata()
                        .withNamespace(namespace)
                        .endMetadata()
                        .build())
                .create();
        kubernetesClient
                .resource(loadYaml(ConfigMap.class, getClass(), "cluster-config.yaml")
                        .edit()
                        .editMetadata()
                        .withNamespace(namespace)
                        .endMetadata()
                        .build())
                .create();

        // create consensus capsule
        List<ConsensusCapsule> consensusCapsules = createConsensusCapsule("node0");

        kubernetesClient.secrets().inNamespace(namespace).withName("node0-keys").edit(secret -> secret.edit()
                .withData(loadYaml(Secret.class, getClass(), "node0-keys.yaml").getData())
                .build());

        // create network initialization
        createNetworkInitialization(consensusCapsules);

        // test with client
        checkAddressBook(HederaSdkClientFactory::directContextCreator);
    }

    @Test
    void testMultiNodeDeployment() throws Exception {
        // create orbit
        createOrbit();

        // populate config and keys
        String internalSuffix = ".consensus";
        String externalSuffix = "-svc";
        kubernetesClient
                .resource(config(
                        loadYaml(ConfigMap.class, getClass(), "cluster-config.yaml")
                                .edit()
                                .editMetadata()
                                .withNamespace(namespace)
                                .endMetadata()
                                .build(),
                        new Node("node0", 1, "node0" + internalSuffix, "node0" + externalSuffix),
                        new Node("node1", 1, "node1" + internalSuffix, "node1" + externalSuffix),
                        new Node("node2", 1, "node2" + internalSuffix, "node2" + externalSuffix)))
                .create();
        kubernetesClient
                .resource(loadYaml(ConfigMap.class, getClass(), "public-keys.yaml")
                        .edit()
                        .editMetadata()
                        .withNamespace(namespace)
                        .endMetadata()
                        .build())
                .create();

        // create consensus capsule
        List<ConsensusCapsule> nodes = createConsensusCapsule("node0", "node1", "node2");

        for (String node : List.of("node0", "node1", "node2")) {
            kubernetesClient
                    .secrets()
                    .inNamespace(namespace)
                    .withName(node + "-keys")
                    .edit(secret -> secret.edit()
                            .withData(loadYaml(Secret.class, getClass(), node + "-keys.yaml")
                                    .getData())
                            .build());
        }

        // create network initialization
        createNetworkInitialization(nodes);

        // test with client
        checkAddressBook(HederaSdkClientFactory::directContextCreator);
        //        fail();
    }

    Orbit createOrbit() {
        Orbit orbit = new OrbitBuilder()
                .withNewMetadata()
                .withName(namespace)
                .endMetadata()
                .withNewSpec()
                .endSpec()
                .build();
        kubernetesClient.resource(orbit).create();

        await().atMost(30, TimeUnit.SECONDS).untilAsserted(() -> {
            assertThat(kubernetesClient.resource(orbit).get()).isNotNull();
        });

        return orbit;
    }

    List<ConsensusCapsule> createConsensusCapsule(String... names) {
        List<ConsensusCapsule> list = new ArrayList<>();
        for (String name : names) {
            final ConsensusCapsule consensusCapsule = new ConsensusCapsuleBuilder()
                    .withNewMetadata()
                    .withName(name)
                    .withNamespace(namespace)
                    .endMetadata()
                    .withNewSpec()
                    .endSpec()
                    .build();
            list.add(kubernetesClient.resource(consensusCapsule).create());
        }
        for (ConsensusCapsule consensusCapsule : list) {
            String name = consensusCapsule.getMetadata().getName();
            await().atMost(30999, TimeUnit.SECONDS).untilAsserted(() -> {
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

    NetworkInitialization createNetworkInitialization(List<ConsensusCapsule> consensusCapsules) {
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
        return networkInitialization;
    }

    List<NodeAddress> addresses(final Client client) throws Exception {
        return NodeAddressBook.fromBytes(
                        new FileContentsQuery().setFileId(FileId.ADDRESS_BOOK).execute(client))
                .getNodeAddresses();
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
                } catch (IllegalStateException e) {
                    if (e.getCause() instanceof BindException) {
                        Thread.sleep(100);
                        continue;
                    } else {
                        throw e;
                    }
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
