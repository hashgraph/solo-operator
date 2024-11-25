// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.orbit;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.operator.reconciler.common.Constants.KEY_TYPE_GOSSIP;
import static com.hedera.solo.operator.reconciler.common.Constants.KEY_TYPE_GRPC_TLS;
import static com.hedera.solo.operator.test.fixtures.Environment.isTestUsingGradleOrIntelliJ;
import static com.hedera.solo.operator.test.fixtures.hederaclient.HederaSdkCheckAddressBook.checkAddressBook;
import static com.hedera.solo.operator.test.fixtures.snapshot.DefaultSnapshot.snapshot;
import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;
import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

import com.hedera.solo.operator.model.commonspec.AddressBookEntry;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.reconciler.common.Metadata;
import com.hedera.solo.operator.test.capsule.ConsensusCapsuleTestAbstract;
import com.hedera.solo.operator.test.fixtures.annotations.SoloTest;
import com.hedera.solo.operator.test.fixtures.hederaclient.HederaSdkClientFactory;
import com.hedera.solo.operator.test.fixtures.snapshot.SnapshotFailureContext;
import com.hedera.solo.operator.test.utils.NodeInfoGenerator;
import com.hedera.solo.operator.test.utils.TestRecords;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.ObjectMeta;
import io.fabric8.kubernetes.api.model.Secret;
import io.fabric8.kubernetes.client.CustomResource;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.javaoperatorsdk.operator.Operator;
import io.quarkus.test.junit.QuarkusTest;
import jakarta.inject.Inject;
import java.lang.invoke.MethodHandles;
import java.time.Duration;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

@QuarkusTest
@SoloTest
public class MultipleOrbitTest {
    @Inject
    protected Operator operator;

    List<Orbit> orbits = new ArrayList<>();

    <T> List<T> copyListWithoutElement(List<T> list, int remove) {
        List<T> copy = new ArrayList<>(list);
        copy.remove(remove);
        return copy;
    }

    @BeforeEach
    void setUp() {
        if (isTestUsingGradleOrIntelliJ()) {
            operator.start();
        }
        cleanup();

        List<String> names = List.of("alpha", "beta", "gamma");
        List<AddressBookEntry> addressBookEntries = new ArrayList<>();
        for (int loop = 0; loop < names.size(); loop++) {
            int id = loop;
            addressBookEntries.add(edit(new AddressBookEntry())
                    .with(builder -> builder.withNodeId((long) id)
                            .withNickName("node" + id)
                            .withSelfName("node" + id)
                            .withWeight(1L)
                            .withInternalHostname("node" + id + ".consensus.multipleorbittest-" + names.get(id))
                            .withInternalPort(50111)
                            .withExternalHostname("node" + id + "-svc.multipleorbittest-" + names.get(id))
                            .withExternalPort(50111)
                            .withNodeAccount("0.0." + (id + 3)))
                    .build());
        }
        Map<String, List<AddressBookEntry>> externalAddresses = new HashMap<>();
        for (int loop = 0; loop < names.size(); loop++) {
            externalAddresses.put(names.get(loop), copyListWithoutElement(addressBookEntries, loop));
        }
        String namespacePrefix =
                MethodHandles.lookup().lookupClass().getSimpleName().toLowerCase();
        names.stream()
                .map(name -> createOrbit(namespacePrefix + "-" + name, externalAddresses.get(name)))
                .forEach(orbits::add);
        await().atMost(Duration.ofSeconds(30)).untilAsserted(() -> {
            for (Orbit orbit : orbits) {
                assertThat(operator.getKubernetesClient()
                                .namespaces()
                                .withName(orbit.getMetadata().getName())
                                .get())
                        .isNotNull();
            }
        });
    }

    @AfterEach
    void tearDown(final SnapshotFailureContext snapshot) {
        snapshot(operator.getKubernetesClient())
                .nonNamespacedResources("operator.solo.hedera.com/v1alpha1")
                .namespaces(orbits.stream()
                        .map(CustomResource::getMetadata)
                        .map(ObjectMeta::getName)
                        .toArray(String[]::new))
                .namespacedResources()
                .logs()
                .exec(
                        "current-address-book",
                        "app=consensus-node",
                        "consensus-node",
                        """
                                find services-hedera/HapiApp2.0/data -type f -name currentAddressBook.txt | \\
                                    xargs -r ls -1t | head -n 1 | xargs -r cat
                                """)
                .file(
                        "app=consensus-node",
                        "consensus-node",
                        "services-hedera/HapiApp2.0/output/hgcaa.log",
                        "services-hedera/HapiApp2.0/output/swirlds.log",
                        "services-hedera/HapiApp2.0/config.txt",
                        "/tmp/volumes/init/hosts-lookup",
                        "/tmp/volumes/init/java.security");
        cleanup();
    }

    void cleanup() {
        orbits.forEach(orbit -> operator.getKubernetesClient().resource(orbit).delete());
        await().atMost(Duration.ofSeconds(60)).untilAsserted(() -> {
            for (Orbit orbit : orbits) {
                assertThat(operator.getKubernetesClient().resource(orbit).get()).isNull();
                assertThat(operator.getKubernetesClient()
                                .namespaces()
                                .withName(orbit.getMetadata().getName())
                                .get())
                        .isNull();
            }
        });
    }

    @Test
    void test() throws Exception {
        setupKeys();
        List<ConsensusCapsule> capsules = createCapsules();
        List<NetworkOperation> networkOperations =
                orbits.stream().map(this::createNetworkOperation).toList();

        // wait for nodes to start
        await().atMost(Duration.ofSeconds(90)).untilAsserted(() -> {
            for (NetworkOperation no : networkOperations) {
                no = operator.getKubernetesClient().resource(no).get();
                assertThat(no.getStatus()).isNotNull();
                assertThat(no.getStatus().getPhase()).isNotNull();
                assertThat(no.getStatus().getPhase()).isEqualTo(NetworkOperationStatus.Phase.Succeeded);
            }
        });

        // verify all nodes have all other nodes in their address book
        for (ConsensusCapsule capsule : capsules) {
            String namespace = capsule.getMetadata().getNamespace();
            checkAddressBook(
                    HederaSdkClientFactory::directContextCreator,
                    operator.getKubernetesClient(),
                    namespace,
                    capsules.size(),
                    capsule.getMetadata().getName());
        }
    }

    Orbit createOrbit(final String name, final List<AddressBookEntry> externalAddresses) {
        Orbit orbit = edit(new Orbit())
                .with(metadata -> metadata.editMetadata().withName(name))
                .with(spec -> spec.editSpec()
                        .editConsensus()
                        .editGenesis()
                        .editAddressBook()
                        .withLedgerId("298")
                        .withNextNodeId("3")
                        .withExternalAddresses(externalAddresses))
                .build();
        return operator.getKubernetesClient().resource(orbit).create();
    }

    void setupKeys() {
        KubernetesClient kubernetesClient = operator.getKubernetesClient();
        long nodeId = 0;
        for (Orbit orbit : orbits) {
            String namespace = orbit.getMetadata().getName();
            TestRecords.NodeInfo node = NodeInfoGenerator.nodeInfo(nodeId++, namespace);

            // load public keys
            for (String cmFile : List.of("public-keys.yaml")) {
                kubernetesClient
                        .resource(loadYaml(ConfigMap.class, ConsensusCapsuleTestAbstract.class, cmFile)
                                .edit()
                                .editMetadata()
                                .withNamespace(namespace)
                                .endMetadata()
                                .build())
                        .create();
            }

            // load private keys
            for (final String keyType : List.of(KEY_TYPE_GOSSIP, KEY_TYPE_GRPC_TLS)) {
                final String nodeKeyFile = String.format("secrets/%s-%s-keys.yaml", node.name(), keyType);
                final Map<String, String> labels = Metadata.prepareConsensusSecretsLabels(
                        orbit.getMetadata().getName(), node.id(), node.accountId(), keyType);
                kubernetesClient
                        .resource(loadYaml(Secret.class, ConsensusCapsuleTestAbstract.class, nodeKeyFile)
                                .edit()
                                .editMetadata()
                                .withNamespace(namespace)
                                .addToLabels(labels)
                                .endMetadata()
                                .build())
                        .create();
            }
        }
    }

    List<ConsensusCapsule> createCapsules() {
        int[] id = new int[] {0};
        return orbits.stream()
                .map(o -> createCapsule(o, "node" + id[0], id[0]++))
                .toList();
    }

    ConsensusCapsule createCapsule(final Orbit orbit, final String name, final long id) {
        ConsensusCapsule capsule = edit(new ConsensusCapsule())
                .with(metadata -> metadata.editMetadata()
                        .withName(name)
                        .withNamespace(orbit.getMetadata().getName()))
                .with(spec -> spec.editSpec()
                        .withOrbit(orbit.getMetadata().getName())
                        .withAccountId("0.0." + (id + 3))
                        .withNodeId(id))
                .build();
        return operator.getKubernetesClient().resource(capsule).create();
    }

    NetworkOperation createNetworkOperation(final Orbit orbit) {
        return operator.getKubernetesClient()
                .resource(edit(new NetworkOperation())
                        .with(metadata -> metadata.editMetadata()
                                .withName("net-op")
                                .withNamespace(orbit.getMetadata().getName()))
                        .with(spec ->
                                spec.editSpec().withOrbit(orbit.getMetadata().getName()))
                        .build())
                .create();
    }
}
