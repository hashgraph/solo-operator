// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.capsule;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.operator.reconciler.common.Constants.COMPONENT_CONSENSUS_NODE;
import static com.hedera.solo.operator.reconciler.common.Constants.KEY_TYPE_GOSSIP;
import static com.hedera.solo.operator.reconciler.common.Constants.KEY_TYPE_GRPC_TLS;
import static com.hedera.solo.operator.reconciler.common.Constants.LABEL_APP;
import static com.hedera.solo.operator.reconciler.common.Constants.LABEL_K8S_COMPONENT;
import static com.hedera.solo.operator.reconciler.common.Constants.LABEL_K8S_MANAGED_BY;
import static com.hedera.solo.operator.reconciler.common.Constants.SOLO_OPERATOR;
import static com.hedera.solo.operator.test.fixtures.coredns.ConfigureCoredns.configureCoredns;
import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;
import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

import com.hedera.solo.operator.beans.DefaultConsensusNodeBean;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsuleSpec;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsuleStatus;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationBuilder;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationSpec;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.PersistentVolumeClaims;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.PodProperties;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.PodPropertiesBuilder;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.podproperties.Containers;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.podproperties.ContainersBuilder;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.podproperties.Volumes;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.podproperties.VolumesBuilder;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.secrets.SecretResources;
import com.hedera.solo.operator.reconciler.common.Metadata;
import com.hedera.solo.operator.test.fixtures.Resources;
import com.hedera.solo.operator.test.fixtures.annotations.SoloTest;
import com.hedera.solo.operator.test.fixtures.hederaclient.HederaSdkCheckAddressBook;
import com.hedera.solo.operator.test.fixtures.hederaclient.HederaSdkClientContextFactory;
import com.hedera.solo.operator.test.fixtures.hederaclient.HederaSdkClientFactory;
import com.hedera.solo.operator.test.fixtures.resource.ResourceLoader;
import com.hedera.solo.operator.test.fixtures.snapshot.SnapshotFailureContext;
import com.hedera.solo.operator.test.setup.ClusterSetupManager;
import com.hedera.solo.operator.test.utils.NodeInfoGenerator;
import com.hedera.solo.operator.test.utils.TestRecords;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.Pod;
import io.fabric8.kubernetes.api.model.Secret;
import io.fabric8.kubernetes.api.model.Service;
import io.fabric8.kubernetes.client.CustomResource;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.javaoperatorsdk.operator.Operator;
import jakarta.inject.Inject;
import java.time.Duration;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.concurrent.TimeUnit;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

/** Abstract class for testing ConsensusCapsule functionality. */
@SoloTest
public abstract class ConsensusCapsuleTestAbstract {
    protected final String namespace = getClass().getSimpleName().toLowerCase();
    protected final String orbit = namespace; // for now orbit and namespace are the same

    @Inject
    DefaultConsensusNodeBean defaultConsensusNode;

    @Inject
    @SuppressWarnings("CdiInjectionPointsInspection")
    KubernetesClient kubernetesClient;

    @Inject
    ClusterSetupManager clusterSetupManager;

    @Inject
    Operator operator;

    /** Set up the test environment before each test. */
    @BeforeEach
    void setup() {
        clusterSetupManager.check();
        configureCoredns(kubernetesClient);

        cleanup();

        // create orbit
        createOrbit();
    }

    /**
     * Clean up the test environment after each test.
     *
     * @param snapshot The snapshot context for the test.
     */
    @AfterEach
    void afterEach(final SnapshotFailureContext snapshot) {
        snapshot.client(kubernetesClient)
                .nonNamespacedResources("operator.solo.hedera.com/v1alpha1")
                .namespaces(namespace)
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
                        "/tmp/volumes/init/hosts-lookup")
                .namespaces("minio-operator", "minio-tenant")
                .namespacedResources()
                .logs()
                .namespaces("kube-system")
                .namespacedResources()
                .logs();
        cleanup();
    }

    /** Clean up the Kubernetes resources. */
    void cleanup() {
        kubernetesClient
                .resources(NetworkOperation.class)
                .inNamespace(namespace)
                .delete();

        kubernetesClient
                .resources(ConsensusCapsule.class)
                .inNamespace(namespace)
                .delete();

        await().atMost(Duration.ofSeconds(30)).untilAsserted(() -> {
            assertThat(kubernetesClient
                            .resources(NetworkOperation.class)
                            .inNamespace(namespace)
                            .list()
                            .getItems())
                    .isEmpty();
            assertThat(kubernetesClient
                            .resources(ConsensusCapsule.class)
                            .inNamespace(namespace)
                            .list()
                            .getItems())
                    .isEmpty();
        });

        Resources.removeByName(kubernetesClient, Orbit.class, namespace);

        await().atMost(Duration.ofSeconds(120))
                .until(() -> kubernetesClient.namespaces().withName(namespace).get() == null);
    }

    private void setupConfigMaps() {
        // create config-maps
        for (String cmFile : List.of("backup-uploader-config.yaml")) {
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

    private void setupSecrets(List<TestRecords.NodeInfo> nodes) {
        final ArrayList<String> secretFiles =
                new ArrayList<>(List.of("secrets/uploader-secrets.yaml", "secrets/mirror-secrets.yaml"));

        for (final String sf : secretFiles) {
            kubernetesClient
                    .resource(loadYaml(Secret.class, getClass(), sf)
                            .edit()
                            .editMetadata()
                            .withNamespace(namespace)
                            .endMetadata()
                            .build())
                    .create();
        }

        for (final TestRecords.NodeInfo node : nodes) {
            for (final String keyType : List.of(KEY_TYPE_GOSSIP, KEY_TYPE_GRPC_TLS)) {
                final String nodeKeyFile = String.format("secrets/%s-%s-keys.yaml", node.name(), keyType);
                final Map<String, String> labels =
                        Metadata.prepareConsensusSecretsLabels(orbit, node.id(), node.accountId(), keyType);

                kubernetesClient
                        .resource(loadYaml(Secret.class, getClass(), nodeKeyFile)
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

    /**
     * Prepares the pod volumes for a consensus capsule.
     *
     * @param node The node information for which the volumes are prepared.
     * @return A Volumes object containing the prepared volumes.
     */
    protected Volumes prepareCapsulePodVolumes(TestRecords.NodeInfo node) {
        return new VolumesBuilder().build(); // default with no volume spec will use empty volumes
    }

    @Test
    void testSingleNodeDeployment() throws Exception {
        this.deployNetwork(1);
    }

    @Test
    void testMultiNodeDeployment() throws Exception {
        this.deployNetwork(3);
    }

    protected List<ConsensusCapsule> deployNetwork(int n) throws Exception {
        final List<Long> nodeIds = NodeInfoGenerator.nodeIds(n);
        final List<TestRecords.NodeInfo> nodes = NodeInfoGenerator.nodes(nodeIds, namespace);
        this.setupConfigMaps();
        this.setupSecrets(nodes);

        // create consensus capsule
        List<ConsensusCapsule> consensusCapsules = createConsensusCapsule(nodes);

        // create network initialization
        createNetworkOperation(consensusCapsules);

        // test with client
        checkAddressBook(HederaSdkClientFactory::directContextCreator);

        return consensusCapsules;
    }

    void createOrbit() {
        final Orbit orbit = edit(loadYaml(Orbit.class, getClass(), "orbit.yaml"))
                .with(metadata -> metadata.editMetadata().withName(namespace))
                .with(ResourceLoader::withResources)
                .build();
        kubernetesClient.resource(orbit).create();

        await().atMost(30, TimeUnit.SECONDS).untilAsserted(() -> {
            assertThat(kubernetesClient.resource(orbit).get()).isNotNull();
            assertThat(kubernetesClient.namespaces().withName(namespace).get()).isNotNull();
        });
    }

    protected ContainersBuilder getContainersBuilder() {
        // return the builder with the consensus-node container
        return new ContainersBuilder().withConsensusNode(defaultConsensusNode.defaultConsensusNode());
    }

    protected Containers prepareConsensusPodContainers() {
        return getContainersBuilder().build();
    }

    protected PersistentVolumeClaims prepareCapsulePVCs(TestRecords.NodeInfo node) {
        return null;
    }

    protected ConsensusCapsule prepareConsensusCapsuleBuilder(final TestRecords.NodeInfo node) {
        final PodProperties consensusPod = new PodPropertiesBuilder()
                .withContainers(this.prepareConsensusPodContainers())
                .withVolumes(this.prepareCapsulePodVolumes(node))
                .build();

        return edit(new ConsensusCapsule())
                .with(ResourceLoader::withResources)
                .with(metadata -> metadata.editMetadata()
                        .withName(node.name())
                        .withNamespace(namespace)
                        .withLabels(Map.of(
                                LABEL_K8S_MANAGED_BY, SOLO_OPERATOR,
                                LABEL_K8S_COMPONENT, COMPONENT_CONSENSUS_NODE)))
                .with(spec -> spec.editSpec()
                        .withPodProperties(consensusPod)
                        .withOrbit(orbit)
                        .withAccountId(node.accountId())
                        .withWeight(1L)
                        .withNodeId(node.id())
                        .withPersistentVolumeClaims(prepareCapsulePVCs(node)))
                .with(secret -> secret.editSpec()
                        .editSecrets()
                        .withSecretResources(edit(new SecretResources())
                                .with(signingCert -> signingCert
                                        .editSigningPublicCertificate()
                                        .withSecretName(node.name() + "-" + KEY_TYPE_GOSSIP + "-keys")
                                        .withKeyName("s-public-" + node.name() + ".pem"))
                                .with(signingKey -> signingKey
                                        .editSigningPrivateKey()
                                        .withSecretName(node.name() + "-" + KEY_TYPE_GOSSIP + "-keys")
                                        .withKeyName("s-private-" + node.name() + ".pem"))
                                .with(tlsCert -> tlsCert.editGrpcTlsPublicCertificate()
                                        .withSecretName(node.name() + "-" + KEY_TYPE_GRPC_TLS + "-keys")
                                        .withKeyName("hedera-" + node.name() + ".crt"))
                                .with(tlsKey -> tlsKey.editGrpcTlsPrivateKey()
                                        .withSecretName(node.name() + "-" + KEY_TYPE_GRPC_TLS + "-keys")
                                        .withKeyName("hedera-" + node.name() + ".key"))
                                .build()))
                .build();
    }

    protected List<Pod> fetchConsensusPods() {
        return kubernetesClient
                .resources(Pod.class)
                .inNamespace(namespace)
                .withLabels(Map.of(LABEL_APP, COMPONENT_CONSENSUS_NODE))
                .list()
                .getItems();
    }

    List<ConsensusCapsule> createConsensusCapsule(List<TestRecords.NodeInfo> nodes) {
        // prepare a list of consensus-capsule for each node
        List<ConsensusCapsule> list = new ArrayList<>();
        for (TestRecords.NodeInfo node : nodes) {
            list.add(kubernetesClient
                    .resource(prepareConsensusCapsuleBuilder(node))
                    .create());
        }

        // setup secrets and services for each consensus-capsule
        for (ConsensusCapsule consensusCapsule : list) {
            String name = consensusCapsule.getMetadata().getName();
            await().atMost(30, TimeUnit.SECONDS).untilAsserted(() -> {
                ConsensusCapsule capsule =
                        kubernetesClient.resource(consensusCapsule).get();
                assertThat(capsule).isNotNull();

                for (final String keyType : List.of(KEY_TYPE_GOSSIP, KEY_TYPE_GRPC_TLS)) {
                    Secret secret = kubernetesClient
                            .secrets()
                            .inNamespace(namespace)
                            .withName("%s-%s-keys".formatted(name, keyType))
                            .get();
                    assertThat(secret).isNotNull();
                }

                Service service = kubernetesClient
                        .services()
                        .inNamespace(namespace)
                        .withName(name + "-svc")
                        .get();
                assertThat(service).isNotNull();
            });
        }

        return list;
    }

    protected void createNetworkOperation(List<ConsensusCapsule> consensusCapsules) throws InterruptedException {
        for (ConsensusCapsule consensusCapsule : consensusCapsules) {
            String name = consensusCapsule.getMetadata().getName();
            ConsensusCapsule capsule =
                    kubernetesClient.resource(consensusCapsule).get();
            assertThat(capsule).isNotNull();
        }

        final NetworkOperation networkGenesis = new NetworkOperationBuilder()
                .withNewMetadata()
                .withName("network-genesis")
                .withNamespace(namespace)
                .withLabels(Map.of(LABEL_K8S_MANAGED_BY, SOLO_OPERATOR))
                .endMetadata()
                .withNewSpec()
                .withOrbit(orbit)
                .withMode(NetworkOperationSpec.Mode.NetworkGenesis)
                .endSpec()
                .withNewStatus()
                .withPhase(NetworkOperationStatus.Phase.Pending)
                .endStatus()
                .build();

        kubernetesClient.resource(networkGenesis).create();

        await().atMost(300, TimeUnit.SECONDS)
                .failFast(() -> {
                    NetworkOperation netInit = kubernetesClient
                            .resources(NetworkOperation.class)
                            .inNamespace(namespace)
                            .withName("network-genesis")
                            .get();
                    assertThat(netInit).isNotNull();
                    if (netInit.getStatus() != null) {
                        assertThat(netInit.getStatus().getPhase()).isNotEqualTo(NetworkOperationStatus.Phase.Failed);
                    }
                })
                .untilAsserted(() -> {
                    NetworkOperation netInit = kubernetesClient
                            .resources(NetworkOperation.class)
                            .inNamespace(namespace)
                            .withName("network-genesis")
                            .get();
                    assertThat(netInit).isNotNull();
                    assertThat(netInit.getStatus()).isNotNull();
                    assertThat(netInit.getStatus().getPhase()).isEqualTo(NetworkOperationStatus.Phase.Succeeded);
                });

        await().atMost(300, TimeUnit.SECONDS).untilAsserted(() -> {
            NetworkOperation netInit = kubernetesClient
                    .resources(NetworkOperation.class)
                    .inNamespace(namespace)
                    .withName("network-genesis")
                    .get();
            assertThat(netInit).isNotNull();
            assertThat(netInit.getStatus()).isNotNull();
            assertThat(netInit.getStatus().getPhase()).isEqualTo(NetworkOperationStatus.Phase.Succeeded);
        });

        // at this point all consensus capsules should be running
        for (ConsensusCapsule consensusCapsule : consensusCapsules) {
            String name = Metadata.prepareNodeName(consensusCapsule.getSpec().getNodeId());
            ConsensusCapsule capsule =
                    kubernetesClient.resource(consensusCapsule).get();
            assertThat(capsule).isNotNull();

            Pod pod = kubernetesClient
                    .resources(Pod.class)
                    .inNamespace(namespace)
                    .withLabel("operator.solo.hedera.com/node-name", name)
                    .list()
                    .getItems()
                    .getFirst();
            assertThat(pod).isNotNull();
            assertThat(pod.getStatus()).isNotNull();
            assertThat(pod.getStatus().getPhase()).isEqualTo("Running");

            assertThat(capsule.getStatus().getPhase()).isEqualTo(ConsensusCapsuleStatus.Phase.Running);
        }
    }

    Long[] consensusNodeIds() {
        return kubernetesClient.resources(ConsensusCapsule.class).inNamespace(namespace).list().getItems().stream()
                .map(CustomResource::getSpec)
                .map(ConsensusCapsuleSpec::getNodeId)
                .toArray(Long[]::new);
    }

    List<String> consensusNodeNames() {
        final Long[] nodeIds = consensusNodeIds();
        final List<String> nodeNames = new ArrayList<>();
        for (Long nodeId : nodeIds) {
            nodeNames.add(Metadata.prepareNodeName(nodeId));
        }

        return nodeNames;
    }

    protected void checkAddressBook(final HederaSdkClientContextFactory hederaSdkClientContextFactory)
            throws AssertionError, Exception {
        List<String> consensusNodeNames = consensusNodeNames();
        HederaSdkCheckAddressBook.checkAddressBook(
                hederaSdkClientContextFactory,
                kubernetesClient,
                namespace,
                consensusNodeNames.size(),
                consensusNodeNames.toArray(String[]::new));
    }
}
