// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.orbit;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.operator.library.k8s.KubeletConfig.clusterDomain;
import static com.hedera.solo.operator.reconciler.common.Constants.COMPONENT_CONSENSUS_NODE;
import static com.hedera.solo.operator.reconciler.common.Constants.KEY_TYPE_GOSSIP;
import static com.hedera.solo.operator.reconciler.common.Constants.KEY_TYPE_GRPC_TLS;
import static com.hedera.solo.operator.reconciler.common.Constants.LABEL_K8S_COMPONENT;
import static com.hedera.solo.operator.reconciler.common.Constants.LABEL_K8S_MANAGED_BY;
import static com.hedera.solo.operator.reconciler.common.Constants.SOLO_OPERATOR;
import static com.hedera.solo.operator.reconciler.common.Metadata.prepareNodeName;
import static com.hedera.solo.operator.test.fixtures.hederaclient.HederaSdkCheckAddressBook.checkAddressBook;
import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;
import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

import com.hedera.solo.operator.beans.DefaultConsensusNodeBean;
import com.hedera.solo.operator.model.commonspec.AddressBookEntry;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationSpec;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.secrets.SecretResources;
import com.hedera.solo.operator.reconciler.common.Metadata;
import com.hedera.solo.operator.test.capsule.ConsensusCapsuleTestAbstract;
import com.hedera.solo.operator.test.fixtures.annotations.SoloTest;
import com.hedera.solo.operator.test.fixtures.hederaclient.HederaSdkClientFactory;
import com.hedera.solo.operator.test.fixtures.resource.ResourceLoader;
import com.hedera.solo.operator.test.fixtures.snapshot.SnapshotFailureContext;
import com.hedera.solo.operator.test.utils.NodeInfoGenerator;
import com.hedera.solo.operator.test.utils.TestRecords;
import io.fabric8.kubernetes.api.model.ObjectMeta;
import io.fabric8.kubernetes.api.model.Secret;
import io.fabric8.kubernetes.client.CustomResource;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.javaoperatorsdk.operator.Operator;
import io.quarkus.test.junit.QuarkusTest;
import jakarta.inject.Inject;
import java.lang.invoke.MethodHandles;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.util.ArrayList;
import java.util.Base64;
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
    DefaultConsensusNodeBean defaultConsensusNode;

    @Inject
    protected Operator operator;

    List<Orbit> orbits = new ArrayList<>();
    Map<String, byte[]> certificates = new HashMap<>();
    Map<String, String> pubKeys = new HashMap<>();

    <T> List<T> copyListWithoutElement(List<T> list, int remove) {
        List<T> copy = new ArrayList<>(list);
        copy.remove(remove);
        return copy;
    }

    @BeforeEach
    void setUp() {
        cleanup();

        List<String> names = List.of("alpha", "beta", "gamma");
        List<AddressBookEntry> addressBookEntries = new ArrayList<>();
        String domain = clusterDomain(operator.getKubernetesClient());
        Base64.Decoder decoder = Base64.getDecoder();
        for (int loop = 0; loop < names.size(); loop++) {
            int id = loop;
            String nodeName = prepareNodeName((long) id);
            Secret secret = loadYaml(
                    Secret.class,
                    ConsensusCapsuleTestAbstract.class,
                    "secrets/" + nodeName + "-" + KEY_TYPE_GOSSIP + "-keys.yaml");
            String publicKey = new String(
                    decoder.decode(secret.getData().get("s-public-" + nodeName + ".pem")), StandardCharsets.UTF_8);
            addressBookEntries.add(edit(new AddressBookEntry())
                    .with(builder -> builder.withNodeId((long) id)
                            .withNickName(nodeName)
                            .withSelfName(nodeName)
                            .withWeight(1L)
                            .withInternalHostname("127.0.0.1")
                            .withInternalPort(50111)
                            .withExternalHostname(nodeName + "-svc.multipleorbittest-" + names.get(id))
                            .withExternalPort(50111)
                            .withNodeAccount("0.0." + (id + 3))
                            .withPublicSigningCertificate(publicKey))
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
        snapshot.client(operator.getKubernetesClient())
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
        for (Orbit orbit : orbits) {
            operator.getKubernetesClient()
                    .resources(ConsensusCapsule.class)
                    .inNamespace(orbit.getMetadata().getName())
                    .delete();
            await().atMost(Duration.ofSeconds(30)).untilAsserted(() -> {
                assertThat(operator.getKubernetesClient()
                                .resources(ConsensusCapsule.class)
                                .inNamespace(orbit.getMetadata().getName())
                                .list()
                                .getItems())
                        .isEmpty();
            });
        }
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
        await().atMost(Duration.ofSeconds(300)).untilAsserted(() -> {
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
                    HederaSdkClientFactory.directContextCreator(certificates, pubKeys),
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
                        .withExternalAddresses(externalAddresses))
                .with(ResourceLoader::withResources)
                .build();
        return operator.getKubernetesClient().resource(orbit).create();
    }

    void setupKeys() {
        KubernetesClient kubernetesClient = operator.getKubernetesClient();
        Base64.Decoder decoder = Base64.getDecoder();
        long nodeId = 0;
        for (Orbit orbit : orbits) {
            String namespace = orbit.getMetadata().getName();
            TestRecords.NodeInfo node = NodeInfoGenerator.nodeInfo(nodeId++, namespace);

            // load private keys
            for (final String keyType : List.of(KEY_TYPE_GOSSIP, KEY_TYPE_GRPC_TLS)) {
                final String nodeKeyFile = String.format("secrets/%s-%s-keys.yaml", node.name(), keyType);
                final Map<String, String> labels = Metadata.prepareConsensusSecretsLabels(
                        orbit.getMetadata().getName(), node.id(), node.accountId(), keyType);
                Secret secret = loadYaml(Secret.class, ConsensusCapsuleTestAbstract.class, nodeKeyFile)
                        .edit()
                        .editMetadata()
                        .withNamespace(namespace)
                        .addToLabels(labels)
                        .endMetadata()
                        .build();
                Map<String, String> data = secret.getData();
                if (KEY_TYPE_GOSSIP.equals(keyType)) {
                    pubKeys.put(
                            node.name(),
                            new String(
                                    decoder.decode(data.get("s-public-" + node.name() + ".pem")),
                                    StandardCharsets.UTF_8));
                } else if (KEY_TYPE_GRPC_TLS.equals(keyType)) {
                    certificates.put(node.name(), decoder.decode(data.get("hedera-" + node.name() + ".crt")));
                }
                secret.setData(Map.of(
                        "tls.crt",
                        data.getOrDefault(
                                "s-public-" + node.name() + ".pem", data.get("hedera-" + node.name() + ".crt")),
                        "tls.key",
                        data.getOrDefault(
                                "s-private-" + node.name() + ".pem", data.get("hedera-" + node.name() + ".key"))));
                kubernetesClient.resource(secret).create();
            }
        }
    }

    List<ConsensusCapsule> createCapsules() {
        long[] id = new long[] {0};
        return orbits.stream()
                .map(o -> createCapsule(o, prepareNodeName(id[0]), id[0]++))
                .toList();
    }

    ConsensusCapsule createCapsule(final Orbit orbit, final String name, final long id) {
        String namespace = orbit.getMetadata().getName();
        ConsensusCapsule capsule = edit(new ConsensusCapsule())
                .with(metadata -> metadata.editMetadata()
                        .withName(name)
                        .withNamespace(namespace)
                        .withLabels(Map.of(
                                LABEL_K8S_MANAGED_BY, SOLO_OPERATOR,
                                LABEL_K8S_COMPONENT, COMPONENT_CONSENSUS_NODE)))
                .with(spec -> spec.editSpec()
                        .withOrbit(orbit.getMetadata().getName())
                        .withAccountId("0.0." + (id + 3))
                        .withWeight(1L)
                        .withNodeId(id)
                        .withInternalAddress("127.0.0.1")
                        .withExternalAddress(prepareNodeName(id) + "-svc." + namespace))
                .with(image -> image.editSpec()
                        .editPodProperties()
                        .editContainers()
                        .withConsensusNode(defaultConsensusNode.defaultConsensusNode()))
                .with(secret -> secret.editSpec()
                        .editSecrets()
                        .withSecretResources(edit(new SecretResources())
                                .with(signingCert -> signingCert
                                        .editSigningPublicCertificate()
                                        .withSecretName(name + "-" + KEY_TYPE_GOSSIP + "-keys"))
                                .with(signingKey -> signingKey
                                        .editSigningPrivateKey()
                                        .withSecretName(name + "-" + KEY_TYPE_GOSSIP + "-keys"))
                                .with(tlsCert -> tlsCert.editGrpcTlsPublicCertificate()
                                        .withSecretName(name + "-" + KEY_TYPE_GRPC_TLS + "-keys"))
                                .with(tlsKey -> tlsKey.editGrpcTlsPrivateKey()
                                        .withSecretName(name + "-" + KEY_TYPE_GRPC_TLS + "-keys"))
                                .build()))
                .with(ResourceLoader::withResources)
                .build();
        return operator.getKubernetesClient().resource(capsule).create();
    }

    NetworkOperation createNetworkOperation(final Orbit orbit) {
        return operator.getKubernetesClient()
                .resource(edit(new NetworkOperation())
                        .with(metadata -> metadata.editMetadata()
                                .withName("net-op")
                                .withLabels(Map.of(LABEL_K8S_MANAGED_BY, SOLO_OPERATOR))
                                .withNamespace(orbit.getMetadata().getName()))
                        .with(spec -> spec.editSpec()
                                .withMode(NetworkOperationSpec.Mode.NetworkGenesis)
                                .withOrbit(orbit.getMetadata().getName()))
                        .build())
                .create();
    }
}
