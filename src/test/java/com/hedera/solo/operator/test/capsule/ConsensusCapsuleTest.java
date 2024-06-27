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

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.dataformat.yaml.YAMLFactory;
import com.hedera.solo.operator.model.commonspec.AddressBookEntry;
import com.hedera.solo.operator.model.commonspec.AddressBookEntryBuilder;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsuleBuilder;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsuleStatus;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitialization;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitializationBuilder;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitializationSpec;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitializationStatus;
import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.model.v1alpha1.OrbitBuilder;
import com.hedera.solo.operator.model.v1alpha1.orbitspec.consensus.genesis.AddressBook;
import com.hedera.solo.operator.test.Environment;
import com.hedera.solo.operator.test.fixtures.Resources;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.ConfigMapBuilder;
import io.fabric8.kubernetes.api.model.ObjectMeta;
import io.fabric8.kubernetes.api.model.Pod;
import io.fabric8.kubernetes.api.model.PodBuilder;
import io.fabric8.kubernetes.api.model.Secret;
import io.fabric8.kubernetes.api.model.Service;
import io.fabric8.kubernetes.api.model.VolumeMount;
import io.fabric8.kubernetes.api.model.VolumeMountBuilder;
import io.fabric8.kubernetes.api.model.apps.Deployment;
import io.fabric8.kubernetes.client.CustomResource;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.fabric8.kubernetes.client.dsl.PodResource;
import io.fabric8.kubernetes.client.dsl.Resource;
import io.javaoperatorsdk.operator.Operator;
import io.quarkus.test.junit.QuarkusTest;
import jakarta.inject.Inject;
import java.lang.invoke.MethodHandles;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.util.ArrayList;
import java.util.Base64;
import java.util.Comparator;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.SortedSet;
import java.util.TreeSet;
import java.util.UUID;
import java.util.concurrent.TimeUnit;
import java.util.function.Function;
import java.util.stream.Collectors;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

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

    String config(ConfigMap configMap, AddressBook external, List<AddressBookEntry> entries) {
        // should probably set the chainId in bootstrap.properties
        StringBuilder sb = new StringBuilder();
        sb.append("swirld, ").append(external.getLedgerId()).append("\n");
        sb.append("app, HederaNode.jar\n");
        SortedSet<AddressBookEntry> addressBookEntries =
                new TreeSet<>(Comparator.comparing(AddressBookEntry::getNodeId));
        addressBookEntries.addAll(external.getExternalAddresses());
        addressBookEntries.addAll(entries);
        for (AddressBookEntry entry : addressBookEntries) {
            sb.append(String.join(
                            ", ",
                            "address",
                            entry.getNodeId().toString(),
                            entry.getNickName(),
                            entry.getSelfName(),
                            entry.getWeight().toString(),
                            entry.getInternalHostname(),
                            entry.getInternalPort().toString(),
                            entry.getExternalHostname(),
                            entry.getExternalPort().toString(),
                            entry.getNodeAccount()))
                    .append("\n");
        }
        sb.append("nextNodeId, ").append(external.getNextNodeId()).append("\n");
        // return configMap.edit().addToData("config.txt", sb.toString()).build();
        return sb.toString();
    }

    List<AddressBookEntry> config(ConfigMap configMap, Node... nodes) {
        // should probably set the chainId in bootstrap.properties
//        String chainId = configMap
//                .getData()
//                .get("bootstrap.properties")
//                .lines()
//                .filter(s -> s.startsWith("contracts.chainId="))
//                .findFirst()
//                .get()
//                .substring(18);
        List<AddressBookEntry> entries = new ArrayList<>();
        long nodeId = 0;
        for (Node node : nodes) {
            entries.add(new AddressBookEntryBuilder()
                    .withNodeId(nodeId)
                    .withNickName(node.name)
                    .withSelfName(node.name)
                    .withWeight(1L)
                    .withInternalHostname(node.internal)
                    .withInternalPort(50111L)
                    .withExternalHostname(node.external)
                    .withExternalPort(50111L)
                    .withNodeAccount("0.0." + (nodeId + 3))
                    .build());
            nodeId++;
        }
        //        return new AddressBookBuilder()
        //                .withExternalAddresses(entries)
        //                .withLedgerId(chainId)
        //                .withNextNodeId(String.valueOf(nodeId))
        //                .build();
        return entries;
    }

    ConfigMap configOld(ConfigMap configMap, Orbit orbit, Node... nodes) {
        AddressBook addressBook = orbit.getSpec().getConsensus().getGenesis().getAddressBook();
        String config = config(configMap, addressBook, config(configMap, nodes));
        return configMap.edit().addToData("config.txt", config).build();
    }

    @Test
    void foo() {
        Orbit orbit = new OrbitBuilder()
                .withNewMetadata()
                .withName(TEST_ORBIT_NAME)
                .endMetadata()
                .withNewSpec()
                .editConsensus()
                .editGenesis()
                .editAddressBook()
                .withLedgerId("321")
                .withNextNodeId("3")
                .endAddressBook()
                .endGenesis()
                .endConsensus()
                .endSpec()
                .build();
        Function<String, String> externalName =
                s -> s + "-svc." + NAMESPACE + ".svc." + orbit.getSpec().getClusterDomain();
        ConfigMap cm = configOld(
                loadYaml(ConfigMap.class, getClass(), "cluster-config.yaml")
                        .edit()
                        .editMetadata()
                        .withNamespace(NAMESPACE)
                        .endMetadata()
                        .build(),
                orbit,
                new Node("node0", 1, "node0", externalName.apply("node0")),
                new Node("node1", 1, "node1", externalName.apply("node1")),
                new Node("node2", 1, "node2", externalName.apply("node2")));
        int i = 1;
    }

    String base64enc(String in) {
        return new String(Base64.getUrlEncoder().encode(in.getBytes(StandardCharsets.UTF_8))).replace('=', '.');
    }

    static class With<T> implements AutoCloseable {
        T val;
        Runnable close;

        public With(T val, Runnable close) {
            this.close = close;
        }

        @Override
        public void close() {
            close.run();
        }
    }

    String run(String namespace, String image, Map<String, String> files, String... args) {
        String name = "run-" + UUID.randomUUID().toString().replace("-", "");
        Resource<ConfigMap> cm = client.configMaps()
                .inNamespace(namespace)
                .resource(new ConfigMapBuilder()
                        .withNewMetadata()
                        .withName(name)
                        .endMetadata()
                        .addToData(files.entrySet().stream()
                                .collect(Collectors.toMap(e -> base64enc(e.getKey()), Map.Entry::getValue)))
                        .build());
        final List<VolumeMount> volumeMounts = files.entrySet().stream()
                .map(e -> new VolumeMountBuilder()
                        .withName(name)
                        .withMountPath(e.getKey())
                        .withSubPath(base64enc(e.getKey()))
                        .build())
                .toList();
        PodResource pod = client.pods()
                .inNamespace(namespace)
                .resource(new PodBuilder()
                        .withNewMetadata()
                        .withName(name)
                        .endMetadata()
                        .withNewSpec()
                        .addNewContainer()
                        .withName(name)
                        .withImage(image)
                        .withArgs(args)
                        .addAllToVolumeMounts(volumeMounts)
                        .endContainer()
                        .addNewVolume()
                        .withName(name)
                        .editConfigMap()
                        .withName(name)
                        .endConfigMap()
                        .endVolume()
                        .withRestartPolicy("Never")
                        .endSpec()
                        .build());
        try (With<ConfigMap> withCm = new With<>(cm.create(), cm::delete);
                With<Pod> withPod = new With<>(pod.create(), pod::delete)) {
            pod.waitUntilCondition(
                    p -> Set.of("Failed", "Succeeded").contains(p.getStatus().getPhase()), 1, TimeUnit.MINUTES);
            return pod.getLog();
        }
    }

    private final Logger log = LoggerFactory.getLogger(MethodHandles.lookup().lookupClass());

    @Test
    void bar() {
        // requires headless service:
        // ipaddr -o show scope global | awk '{gsub("/.*", ""); print "nslookup " $4}' | sh | awk '/name = .*\.svc\..*/{gsub(".*\.svc\.", ""); print}'
        // kubectl run -it --image alpine:latest --restart=Never --rm foo2 -- sh -c "sleep 1; $(printf "ipaddr -o show scope global | awk '{gsub(\"/.*\", \"\"); print \"nslookup \" \$4}' | sh | awk '/name = .*\.svc\..*/{gsub(\".*\.svc\.\", \"\"); print}'")"
        String out = run("default", "alpine:3.20.1", Map.of("ab/c", "foo", "cde", "bat"), "cat", "/etc/resolv.conf");
        log.info("out: {}", out);
        int i = 1;
    }

    @Test
    void readConfig() throws JsonProcessingException {
//        client.nodes().list().getItems();
//        client.raw("/api/v1/nodes/kind-control-plane/proxy/configz");
        YAMLFactory yamlFactory = new YAMLFactory();
        ObjectMapper mapper = new ObjectMapper(yamlFactory);
        String yaml = client.configMaps().inNamespace("kube-system").withName("kubeadm-config").get().getData().get("ClusterConfiguration");
        String dnsDomain = mapper.readTree(yaml).get("networking").get("dnsDomain").textValue();
        log.info("dnsDomain: {}", dnsDomain);
    }

    @Test
    void testSingleNodeDeployment() {
        // create orbit
        createOrbit(1);

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
        List<ConsensusCapsule> consensusCapsules = createConsensusCapsule(new NodeX(0, "node0"));

        // create network initialization
        createNetworkInitialization(consensusCapsules);
    }

    @Test
    void testMultiNodeDeployment() {
        // create orbit
        Orbit orbit = createOrbit(3);

        // populate config and keys
        Function<String, String> externalName =
                s -> s + "-svc." + NAMESPACE + ".svc." + orbit.getSpec().getClusterDomain();
        client.resource(configOld(
                        loadYaml(ConfigMap.class, getClass(), "cluster-config.yaml")
                                .edit()
                                .editMetadata()
                                .withNamespace(NAMESPACE)
                                .endMetadata()
                                .build(),
                        orbit,
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
        List<ConsensusCapsule> nodes =
                createConsensusCapsule(new NodeX(0, "node0"), new NodeX(1, "node1"), new NodeX(2, "node2"));

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

    Orbit createOrbit(int nextNodeId) {
        Orbit orbit = client.resource(new OrbitBuilder()
                        .withNewMetadata()
                        .withName(TEST_ORBIT_NAME)
                        .endMetadata()
                        .withNewSpec()
                        .editConsensus()
                        .editGenesis()
                        .editAddressBook()
                        .withLedgerId("321")
                        .withNextNodeId(String.valueOf(nextNodeId))
                        .endAddressBook()
                        .endGenesis()
                        .endConsensus()
                        .endSpec()
                        .build())
                .create();

        await().atMost(30, TimeUnit.SECONDS).untilAsserted(() -> {
            assertThat(client.resource(orbit).get()).isNotNull();
        });

        return orbit;
    }

    record NodeX(long id, String name) {}

    List<ConsensusCapsule> createConsensusCapsule(NodeX... nodes) {
        List<ConsensusCapsule> list = new ArrayList<>();
        for (NodeX node : nodes) {
            final ConsensusCapsule consensusCapsule = new ConsensusCapsuleBuilder()
                    .withNewMetadata()
                    .withName(node.name)
                    .withNamespace(NAMESPACE)
                    .endMetadata()
                    .withNewSpec()
                    .withNodeId(node.id)
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
                .withCapsules(consensusCapsules.stream()
                        .map(CustomResource::getMetadata)
                        .map(ObjectMeta::getName)
                        .toList())
                .endSpec()
                .withNewStatus()
                .endStatus()
                .build();
        client.resource(networkInitialization).create();

        await().atMost(90, TimeUnit.SECONDS).untilAsserted(() -> {
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
