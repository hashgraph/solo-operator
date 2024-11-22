// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.capsule;

import static com.hedera.solo.operator.reconciler.common.Constants.CONSENSUS_NODE;
import static com.hedera.solo.operator.reconciler.common.Constants.KEY_TYPE_GOSSIP;
import static com.hedera.solo.operator.reconciler.common.Constants.KEY_TYPE_GRPC_TLS;
import static com.hedera.solo.operator.reconciler.common.Constants.LABEL_APP;
import static com.hedera.solo.operator.test.fixtures.Environment.isTestUsingGradleOrIntelliJ;
import static com.hedera.solo.operator.test.fixtures.coredns.ConfigureCoredns.configureCoredns;
import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;
import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsuleBuilder;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsuleStatus;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationBuilder;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationSpec;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.model.v1alpha1.OrbitBuilder;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.PersistentVolumeClaims;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.PersistentVolumeClaimsBuilder;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.PodProperties;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.PodPropertiesBuilder;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.podproperties.Containers;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.podproperties.ContainersBuilder;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.podproperties.Volumes;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.podproperties.VolumesBuilder;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.podproperties.containers.ConsensusNode;
import com.hedera.solo.operator.reconciler.common.Constants;
import com.hedera.solo.operator.test.fixtures.Resources;
import com.hedera.solo.operator.test.fixtures.annotations.SoloTest;
import com.hedera.solo.operator.test.fixtures.hederaclient.HederaSdkCheckAddressBook;
import com.hedera.solo.operator.test.fixtures.hederaclient.HederaSdkClientContextFactory;
import com.hedera.solo.operator.test.fixtures.hederaclient.HederaSdkClientFactory;
import com.hedera.solo.operator.test.fixtures.snapshot.SnapshotFailureContext;
import com.hedera.solo.operator.test.setup.ClusterSetupManager;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.ObjectMeta;
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

@SoloTest
public abstract class ConsensusCapsuleTestAbstract {
    protected final String namespace = getClass().getSimpleName().toLowerCase();
    protected final String svcNameSuffix = ".%s.%s".formatted(namespace, "svc.cluster.local");

    @Inject
    @SuppressWarnings("CdiInjectionPointsInspection")
    KubernetesClient kubernetesClient;

    @Inject
    ClusterSetupManager clusterSetupManager;

    @Inject
    Operator operator;

    @BeforeEach
    void setup() {
        clusterSetupManager.check();
        configureCoredns(kubernetesClient);
        if (isTestUsingGradleOrIntelliJ()) {
            operator.start();
        }

        cleanup();

        // create orbit
        createOrbit();
    }

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

    void cleanup() {
        kubernetesClient
                .resources(NetworkOperation.class)
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

    protected record NodeInfo(Long id, String name, String accountId, int weight, String internal, String external) {}

    private List<NodeInfo> prepareNodes(List<Long> nodeIds) {
        List<NodeInfo> nodes = new ArrayList<>();
        int accStart = Constants.HEDERA_ACCOUNT_START;
        for (Long nodeId : nodeIds) {
            String nodeName = Constants.NODE_NAME_FORMAT.formatted(nodeId);
            nodes.add(new NodeInfo(
                    nodeId,
                    nodeName,
                    String.format("0.0.%d", accStart++),
                    1,
                    String.format("%s.consensus%s", nodeName, svcNameSuffix),
                    String.format("%s-svc%s", nodeName, svcNameSuffix)));
        }

        return nodes;
    }

    private void setupConfigMaps() {
        // create config-maps
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

        for (final NodeInfo node : nodes) {
            for (final String keyType : List.of(KEY_TYPE_GOSSIP, KEY_TYPE_GRPC_TLS)) {
                final String nodeKeyFile = String.format("secrets/%s-%s-keys.yaml", node.name, keyType);
                kubernetesClient
                        .resource(loadYaml(Secret.class, getClass(), nodeKeyFile)
                                .edit()
                                .editMetadata()
                                .withNamespace(namespace)
                                .addToLabels("consensus-capsule", node.name)
                                .addToLabels("key-type", keyType)
                                .endMetadata()
                                .build())
                        .create();
            }
        }
    }

    protected Volumes prepareCapsulePodVolumes(NodeInfo node) {
        return new VolumesBuilder().build(); // default with no volume spec will use empty volumes
    }

    protected List<Long> prepareNodeIds(int count) {
        long nodeId = 0L;
        List<Long> nodeIds = new ArrayList<>();
        while (count-- > 0) {
            nodeIds.add(nodeId++);
        }

        return nodeIds;
    }

    @Test
    void testSingleNodeDeployment() throws Exception {
        final List<Long> nodeIds = prepareNodeIds(1);
        final List<NodeInfo> nodes = prepareNodes(nodeIds);
        this.setupConfigMaps();
        this.setupSecrets(nodes);

        // create consensus capsule
        List<ConsensusCapsule> consensusCapsules = createConsensusCapsule(nodes);

        // create network initialization
        createNetworkOperation(consensusCapsules);

        // test with client
        checkAddressBook(HederaSdkClientFactory::directContextCreator);
    }

    @Test
    void testMultiNodeDeployment() throws Exception {
        final List<Long> nodeIds = prepareNodeIds(3);
        final List<NodeInfo> nodes = prepareNodes(nodeIds);
        this.setupConfigMaps();
        this.setupSecrets(nodes);

        // create consensus capsule
        List<ConsensusCapsule> consensusCapsules = createConsensusCapsule(nodes);

        // create network initialization
        createNetworkOperation(consensusCapsules);

        // test with client
        checkAddressBook(HederaSdkClientFactory::directContextCreator);
    }

    void createOrbit() {
        Orbit orbit = new OrbitBuilder()
                .withNewMetadata()
                .withName(namespace)
                .endMetadata()
                .editSpec()
                .editOrbitspecConsensus()
                .editGenesis()
                .editAddressBook()
                .withLedgerId("298")
                .endAddressBook()
                .endGenesis()
                .endOrbitspecConsensus()
                .endSpec()
                .build();

        kubernetesClient.resource(orbit).create();

        await().atMost(30, TimeUnit.SECONDS).untilAsserted(() -> {
            assertThat(kubernetesClient.resource(orbit).get()).isNotNull();
            assertThat(kubernetesClient.namespaces().withName(namespace).get()).isNotNull();
        });
    }

    protected ContainersBuilder getContainersBuilder() {
        // return the builder with the consensus-node container
        final ConsensusNode consensusNode = loadYaml(ConsensusNode.class, getClass(), "containers/consensus-node.yaml");
        return new ContainersBuilder().withConsensusNode(consensusNode);
    }

    protected Containers prepareConsensusPodContainers() {
        return getContainersBuilder().build();
    }

    protected PersistentVolumeClaims prepareCapsulePVCs(NodeInfo node) {
        return new PersistentVolumeClaimsBuilder().build();
    }

    protected ConsensusCapsuleBuilder prepareConsensusCapsuleBuilder(NodeInfo node) {
        final PodProperties consensusPod = new PodPropertiesBuilder()
                .withContainers(this.prepareConsensusPodContainers())
                .withVolumes(this.prepareCapsulePodVolumes(node))
                .build();

        return new ConsensusCapsuleBuilder()
                .withNewMetadata()
                .withName(node.name)
                .withNamespace(namespace)
                .endMetadata()
                .withNewSpec()
                .withPodProperties(consensusPod)
                .withOrbit(namespace) // for now, namespace and orbit must match
                .withAccountId(node.accountId)
                .withNodeId(node.id)
                .withPersistentVolumeClaims(prepareCapsulePVCs(node))
                .endSpec();
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
        // prepare a list of consensus-capsule for each node
        List<ConsensusCapsule> list = new ArrayList<>();
        for (NodeInfo node : nodes) {
            list.add(kubernetesClient
                    .resource(prepareConsensusCapsuleBuilder(node).build())
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
                assertThat(capsule.getSpec().getEligibleForDeployment()).isFalse();
            });
        }

        return list;
    }

    void createNetworkOperation(List<ConsensusCapsule> consensusCapsules) {
        for (ConsensusCapsule consensusCapsule : consensusCapsules) {
            String name = consensusCapsule.getMetadata().getName();
            ConsensusCapsule capsule =
                    kubernetesClient.resource(consensusCapsule).get();
            assertThat(capsule).isNotNull();
            assertThat(capsule.getSpec().getEligibleForDeployment()).isFalse();

            Pod pod = kubernetesClient
                    .resources(Pod.class)
                    .inNamespace(namespace)
                    .withName(name + "-pod")
                    .get();
            assertThat(pod).isNull();
        }

        final NetworkOperation networkOperation = new NetworkOperationBuilder()
                .withNewMetadata()
                .withName("network-operation")
                .withNamespace(namespace)
                .endMetadata()
                .withNewSpec()
                .withMode(NetworkOperationSpec.Mode.FirstInitialization)
                .endSpec()
                .withNewStatus()
                .endStatus()
                .build();
        kubernetesClient.resource(networkOperation).create();

        await().atMost(300, TimeUnit.SECONDS)
                .failFast(() -> {
                    NetworkOperation netOp =
                            kubernetesClient.resource(networkOperation).get();
                    assertThat(netOp).isNotNull();
                    if (netOp.getStatus() != null) {
                        assertThat(netOp.getStatus().getPhase()).isNotEqualTo(NetworkOperationStatus.Phase.Failed);
                    }
                })
                .untilAsserted(() -> {
                    NetworkOperation netInit =
                            kubernetesClient.resource(networkOperation).get();
                    assertThat(netInit).isNotNull();
                    assertThat(netInit.getStatus()).isNotNull();
                    assertThat(netInit.getStatus().getPhase()).isEqualTo(NetworkOperationStatus.Phase.Succeeded);
                    for (ConsensusCapsule consensusCapsule : consensusCapsules) {
                        String name = consensusCapsule.getMetadata().getName();
                        ConsensusCapsule capsule =
                                kubernetesClient.resource(consensusCapsule).get();
                        assertThat(capsule).isNotNull();
                        assertThat(capsule.getSpec().getEligibleForDeployment()).isTrue();

                        Pod pod = kubernetesClient
                                .resources(Pod.class)
                                .inNamespace(namespace)
                                .withName(name + "-pod")
                                .get();
                        assertThat(pod).isNotNull();
                        assertThat(pod.getStatus()).isNotNull();
                        assertThat(pod.getStatus().getPhase()).isEqualTo("Running");

                        assertThat(capsule.getStatus().getPhase()).isEqualTo(ConsensusCapsuleStatus.Phase.Running);
                    }
                });
    }

    String[] consensusNodeNames() {
        return kubernetesClient.resources(ConsensusCapsule.class).inNamespace(namespace).list().getItems().stream()
                .map(CustomResource::getMetadata)
                .map(ObjectMeta::getName)
                .toArray(String[]::new);
    }

    protected void checkAddressBook(final HederaSdkClientContextFactory hederaSdkClientContextFactory)
            throws AssertionError, Exception {
        String[] consensusNodeNames = consensusNodeNames();
        HederaSdkCheckAddressBook.checkAddressBook(
                hederaSdkClientContextFactory,
                kubernetesClient,
                namespace,
                consensusNodeNames.length,
                consensusNodeNames);
    }
}
