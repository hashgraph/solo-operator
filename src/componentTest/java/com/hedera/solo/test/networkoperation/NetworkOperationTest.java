// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.test.networkoperation;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.operator.model.v1alpha1.NetworkOperationSpec.Mode.NetworkGenesis;
import static com.hedera.solo.operator.reconciler.common.Constants.LABEL_K8S_COMPONENT;
import static com.hedera.solo.operator.reconciler.common.Constants.LABEL_K8S_MANAGED_BY;
import static com.hedera.solo.operator.reconciler.common.Constants.OPERATOR_PREFIX;
import static com.hedera.solo.operator.reconciler.common.Constants.SOLO_OPERATOR;
import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

import com.hedera.solo.operator.library.k8s.CustomResourceManager;
import com.hedera.solo.operator.model.commonspec.KeyReference;
import com.hedera.solo.operator.model.commonspec.KeyReferenceBuilder;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.secrets.SecretResources;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.secrets.SecretResourcesBuilder;
import com.hedera.solo.operator.reconciler.common.ConditionStatus;
import com.hedera.solo.operator.reconciler.common.Constants;
import com.hedera.solo.operator.reconciler.common.Metadata;
import com.hedera.solo.operator.reconciler.networkoperation.check.NetworkCheckProcessor;
import com.hedera.solo.operator.reconciler.networkoperation.check.NetworkCheckTask;
import com.hedera.solo.operator.reconciler.networkoperation.genesis.NetworkGenesisProcessor;
import com.hedera.solo.operator.reconciler.networkoperation.genesis.NetworkGenesisTask;
import com.hedera.solo.operator.reconciler.networkoperation.genesis.steps.CheckConsensusNodeSecretsStep;
import com.hedera.solo.operator.reconciler.networkoperation.node.NodeOperationProcessor;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.NetworkUpgradeProcessor;
import com.hedera.solo.operator.test.fixtures.annotations.SoloTest;
import com.hedera.solo.operator.test.fixtures.resource.ResourceLoader;
import com.hedera.solo.operator.test.fixtures.snapshot.SnapshotFailureContext;
import com.hedera.solo.test.framework.AbstractOperatorTest;
import io.fabric8.kubernetes.api.model.Condition;
import io.fabric8.kubernetes.api.model.Event;
import io.fabric8.kubernetes.api.model.ObjectReference;
import io.fabric8.kubernetes.api.model.PodBuilder;
import io.fabric8.kubernetes.api.model.PodList;
import io.fabric8.kubernetes.api.model.Secret;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.javaoperatorsdk.operator.api.config.ControllerConfigurationOverrider;
import io.javaoperatorsdk.operator.junit.LocallyRunOperatorExtension;
import java.util.ArrayList;
import java.util.Collection;
import java.util.List;
import java.util.Map;
import java.util.concurrent.TimeUnit;
import java.util.stream.Collectors;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.mockito.Mockito;

@SoloTest
public class NetworkOperationTest extends AbstractOperatorTest {

    /**
     * Prepare a custom resource manager with mocked methods
     *
     * <p>We are mocking methods that are required for example the pods method; otherwise delegating to the actual
     * Kubernetes client
     *
     * @param client Kubernetes client
     * @return Custom resource manager with mocked methods
     */
    private CustomResourceManager prepareCustomResourceManager(KubernetesClient client) {
        final CustomResourceManager delegate = CustomResourceManager.with(client);

        final CustomResourceManager customResourceManager = Mockito.mock(CustomResourceManager.class);
        Mockito.when(customResourceManager.orbitByName(Mockito.anyString())).thenAnswer(invocation -> {
            final String namespace = operator().getNamespace();
            return delegate.orbitByName(namespace);
        });

        Mockito.when(customResourceManager.configMapByName(Mockito.anyString(), Mockito.anyString()))
                .thenAnswer(invocation -> {
                    final String namespace = invocation.getArgument(0);
                    final String name = invocation.getArgument(1);
                    return delegate.configMapByName(namespace, name);
                });

        Mockito.when(customResourceManager.secretByName(Mockito.anyString(), Mockito.anyString()))
                .thenAnswer(invocation -> {
                    final String namespace = invocation.getArgument(0);
                    final String name = invocation.getArgument(1);
                    return delegate.secretByName(namespace, name);
                });

        Mockito.when(customResourceManager.networkOperations()).thenAnswer(invocation -> {
            final String namespace = operator().getNamespace();
            return delegate.networkOperations(namespace);
        });

        Mockito.when(customResourceManager.networkOperations(Mockito.anyString()))
                .thenAnswer(invocation -> {
                    final String namespace = invocation.getArgument(0);
                    return delegate.networkOperations(namespace);
                });

        Mockito.when(customResourceManager.consensusCapsules(Mockito.anyString(), Mockito.anyMap()))
                .thenAnswer(invocation -> {
                    final String namespace = invocation.getArgument(0);
                    return delegate.consensusCapsules(namespace);
                });

        Mockito.when(customResourceManager.consensusCapsules(Mockito.anyString()))
                .thenAnswer(invocation -> {
                    final String namespace = invocation.getArgument(0);
                    return delegate.consensusCapsules(namespace);
                });

        Mockito.when(customResourceManager.triggerEvent(Mockito.any()))
                .thenAnswer(invocation -> delegate.triggerEvent(invocation.getArgument(0)));

        Mockito.when(customResourceManager.pods(Mockito.anyString(), Mockito.anyMap()))
                .thenAnswer(invocation -> {
                    final String namespace = invocation.getArgument(0);
                    final PodList podList = new PodList();

                    List<Long> capsuleNodeIds = prepareNodeIds(3);
                    for (Long nodeId : capsuleNodeIds) {
                        podList.getItems()
                                .add(new PodBuilder()
                                        .withNewMetadata()
                                        .withName(Metadata.prepareNodeName(nodeId))
                                        .withNamespace(namespace)
                                        .endMetadata()
                                        .withNewStatus()
                                        .withPhase("Running")
                                        .withPodIP("10.0.0.%d".formatted(nodeId + 1))
                                        .endStatus()
                                        .build());
                    }

                    return podList;
                });

        return customResourceManager;
    }

    @Override
    protected LocallyRunOperatorExtension.Builder configure(final KubernetesClient client) {
        final CustomResourceManager customResourceManager = prepareCustomResourceManager(client);
        final NetworkCheckProcessor networkCheckProcessor =
                new NetworkCheckProcessor(new NetworkCheckTask(customResourceManager));
        final NetworkGenesisProcessor networkGenesisProcessor =
                new NetworkGenesisProcessor(new NetworkGenesisTask(customResourceManager, networkCheckProcessor));
        final NetworkUpgradeProcessor networkUpgradeProcessor = Mockito.mock(NetworkUpgradeProcessor.class);
        final NodeOperationProcessor nodeOperationProcessor = Mockito.mock(NodeOperationProcessor.class);
        return builder(client)
                .withReconciler(
                        new NetworkOperationTestReconciler(
                                customResourceManager,
                                networkGenesisProcessor,
                                networkUpgradeProcessor,
                                networkCheckProcessor,
                                nodeOperationProcessor),
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

    private List<Long> prepareNodeIds(int count) {
        List<Long> nodeIds = new ArrayList<>();
        for (int i = 0; i < count; i++) {
            nodeIds.add((long) i);
        }
        return nodeIds;
    }

    private List<String> prepareNodeNames(List<Long> nodeIds) {
        List<String> nodeNames = new ArrayList<>();
        for (Long nodeId : nodeIds) {
            nodeNames.add(Metadata.prepareNodeName(nodeId));
        }

        return nodeNames;
    }

    private String networkOperationName(int i) {
        return "init-%02d".formatted(i); // prepare name with padding so that it can be ordered by name
    }

    @Test
    void oneNetworkOperationAtATime() {
        int total = 5;
        List<Long> capsuleNodeIds = prepareNodeIds(3);
        List<String> capsuleNames = prepareNodeNames(capsuleNodeIds);

        capsuleNodeIds.forEach(this::createCapsule);

        for (int i = 0; i < total; ++i) {
            String name = networkOperationName(i);
            createNetworkOperation(name, false);
            await().atMost(60, TimeUnit.SECONDS).untilAsserted(() -> {
                verifyNetworkOperationSucceeded(name, true);
            });
        }
    }

    @Test
    void manyNetworkOperationsAtOnceWithDeletions() {
        int total = 10;
        int noUpdatePeriod = 3;
        List<Long> capsuleNodeIds = prepareNodeIds(3);

        capsuleNodeIds.forEach(this::createCapsule);

        for (int i = 0; i < total; ++i) {
            String name = networkOperationName(i);
            createNetworkOperation(name, 0 == i % noUpdatePeriod);
        }

        for (int i = 0; i < total; i += noUpdatePeriod) {
            String name = networkOperationName(i);
            deleteNetworkOperationStuckInPending(name);
        }

        for (int i = 0; i < total; ++i) {
            final int nodeId = i;
            await().atMost(60, TimeUnit.SECONDS).untilAsserted(() -> {
                String name = networkOperationName(nodeId);
                verifyNetworkOperationSucceeded(name, 0 != nodeId % noUpdatePeriod);
            });
        }
    }

    @Test
    void failNetworkOperation() {
        List<Long> capsuleNodeIds = prepareNodeIds(3);
        capsuleNodeIds.forEach(nodeId -> createCapsule(
                nodeId,
                Map.of(
                        LABEL_K8S_MANAGED_BY,
                        SOLO_OPERATOR,
                        LABEL_K8S_COMPONENT,
                        Constants.COMPONENT_CONSENSUS_NODE,
                        "fail",
                        "true"),
                null));

        createNetworkOperation("init", false);
        await().atMost(60, TimeUnit.SECONDS).untilAsserted(() -> {
            NetworkOperation no = operator().get(NetworkOperation.class, "init");
            assertThat(no).isNotNull();
            assertThat(no.getStatus()).isNotNull();
            assertThat(no.getStatus().getPhase()).isEqualTo(NetworkOperationStatus.Phase.Failed);
            Map<String, List<Condition>> conditions =
                    no.getStatus().getConditions().stream().collect(Collectors.groupingBy(Condition::getType));

            assertThat(conditions.values().stream().flatMap(Collection::stream))
                    .size()
                    .isEqualTo(4);

            Condition valid = conditions.get("CheckConsensusSecrets").getFirst();
            assertThat(valid.getStatus()).isEqualTo(ConditionStatus.True);

            Condition ready = conditions.get("ConsensusCapsuleReady").getFirst();
            assertThat(ready.getStatus()).isEqualTo(ConditionStatus.False);
            assertThat(ready.getMessage()).isEqualTo("Capsule phases: {node1=Failed, node2=Failed, node3=Failed}");
        });
    }

    @Test
    void validSecretResources() {
        KeyReference keyReference = createSecret();
        SecretResources secretResources = new SecretResourcesBuilder()
                .withGrpcTlsPrivateKey(keyReference)
                .withGrpcTlsPublicCertificate(keyReference)
                .withSigningPrivateKey(keyReference)
                .withSigningPublicCertificate(keyReference)
                .build();

        secretsValid(secretResources, true);
    }

    @Test
    void secretResourcesInvalidSecret() {
        KeyReference keyReference = createSecret();
        secretsInvalid(new SecretResourcesBuilder()
                .withGrpcTlsPrivateKey(keyReference)
                .withGrpcTlsPublicCertificate(keyReference)
                .withSigningPrivateKey(keyReference)
                .withSigningPublicCertificate(new KeyReferenceBuilder()
                        .withSecretName("invalid")
                        .withKeyName(keyReference.getKeyName())
                        .build())
                .build());
    }

    @Test
    void secretResourcesInvalidKey() {
        KeyReference keyReference = createSecret();
        secretsInvalid(new SecretResourcesBuilder()
                .withGrpcTlsPrivateKey(keyReference)
                .withGrpcTlsPublicCertificate(keyReference)
                .withSigningPrivateKey(keyReference)
                .withSigningPublicCertificate(new KeyReferenceBuilder()
                        .withSecretName(keyReference.getSecretName())
                        .withKeyName("invalid")
                        .build())
                .build());
    }

    KeyReference createSecret() {
        Secret secret = operator()
                .create(edit(new Secret())
                        .with(metadata -> metadata.editMetadata()
                                .withGenerateName("secret-")
                                .withLabels(Map.of(
                                        LABEL_K8S_MANAGED_BY,
                                        SOLO_OPERATOR,
                                        LABEL_K8S_COMPONENT,
                                        Constants.COMPONENT_SECRET)))
                        .with(data -> data.withData(Map.of("key", "dmFsdWU=")))
                        .build());
        await().atMost(30, TimeUnit.SECONDS).untilAsserted(() -> {
            assertThat(operator().get(Secret.class, secret.getMetadata().getName()))
                    .isNotNull();
        });
        return new KeyReferenceBuilder()
                .withSecretName(secret.getMetadata().getName())
                .withKeyName("key")
                .build();
    }

    NetworkOperation secretsValid(final SecretResources secretResources, final boolean valid) {
        List<Long> capsuleNodeIds = prepareNodeIds(3);
        for (Long nodeId : capsuleNodeIds) {
            createCapsule(nodeId, secretResources);
        }

        String name = "init";
        NetworkOperation networkOperation = createNetworkOperation(name, false);
        await().atMost(60, TimeUnit.SECONDS).untilAsserted(() -> {
            NetworkOperation no = operator().get(NetworkOperation.class, name);
            assertThat(no).isNotNull();
            assertThat(no.getStatus()).isNotNull();
            assertThat(no.getStatus().getConditions()).isNotNull();
            List<Condition> conditions = no.getStatus().getConditions().stream()
                    .filter(c -> CheckConsensusNodeSecretsStep.CONDITION_TYPE.equals(c.getType()))
                    .toList();
            assertThat(conditions.size()).isEqualTo(1);
            Condition condition = conditions.getFirst();
            assertThat(condition.getStatus()).isEqualTo(valid ? ConditionStatus.True : ConditionStatus.False);
            if (!valid) {
                // check that at least one condition no with reason SECRET_VALID is present and has status False
                boolean found = false;
                for (Condition c : no.getStatus().getConditions()) {
                    if (CheckConsensusNodeSecretsStep.SECRETS_VALID.equals(c.getReason())) {
                        if (c.getStatus().equals(ConditionStatus.False)) {
                            found = true;
                            break;
                        }
                    }
                }
                assertThat(found).isTrue();
            }
        });
        return networkOperation;
    }

    void secretsInvalid(final SecretResources secretResources) {
        NetworkOperation no = secretsValid(secretResources, false);
        await().atMost(30, TimeUnit.SECONDS).untilAsserted(() -> {
            List<Event> events = operator()
                    .getKubernetesClient()
                    .resources(Event.class)
                    .inNamespace(operator().getNamespace())
                    .withInvolvedObject(new ObjectReference(
                            no.getApiVersion(),
                            null,
                            no.getKind(),
                            no.getMetadata().getName(),
                            no.getMetadata().getNamespace(),
                            null,
                            no.getMetadata().getUid()))
                    .list()
                    .getItems();
            assertThat(events.size()).isEqualTo(1);
            Event event = events.getFirst();
            assertThat(event.getReportingComponent()).isEqualTo("solo-operator");
            assertThat(event.getType()).isEqualTo("Error");
            assertThat(event.getReason()).isEqualTo("Failed");
            assertThat(event.getMessage()).contains("Secrets validation failed");
        });
    }

    NetworkOperation createNetworkOperation(final String name, final boolean noUpdate) {
        final String orbitName = operator().getNamespace(); // for now namespace and orbit name are the same
        final Map<String, String> labels = Map.of(LABEL_K8S_MANAGED_BY, SOLO_OPERATOR);
        return operator()
                .create(edit(new NetworkOperation())
                        .with(metadata -> metadata.editMetadata()
                                .withName(name)
                                .addToLabels(labels)
                                .addToLabels("no-update", String.valueOf(noUpdate)))
                        .with(spec -> spec.editSpec().withMode(NetworkGenesis).withOrbit(orbitName))
                        .build());
    }

    void deleteNetworkOperationStuckInPending(final String name) {
        await().atMost(30, TimeUnit.SECONDS).untilAsserted(() -> {
            NetworkOperation no = operator().get(NetworkOperation.class, name);
            assertThat(no).isNotNull();
            assertThat(no.getStatus()).isNotNull();
            assertThat(no.getStatus().getPhase()).isEqualTo(NetworkOperationStatus.Phase.Pending);
        });
        sleep(1000);
        assertThat(operator().get(NetworkOperation.class, name).getStatus().getPhase())
                .isEqualTo(NetworkOperationStatus.Phase.Pending);
        operator().delete(operator().get(NetworkOperation.class, name));
    }

    void verifyNetworkOperationSucceeded(final String name, final boolean exists) {
        NetworkOperation no = operator().get(NetworkOperation.class, name);
        if (!exists) {
            assertThat(no).isNull();
        } else {
            assertThat(no).isNotNull();
            assertThat(no.getStatus()).isNotNull();
            assertThat(no.getStatus().getPhase()).isEqualTo(NetworkOperationStatus.Phase.Succeeded);
            List<Condition> conditions = no.getStatus().getConditions();
            assertThat(conditions.stream().map(Condition::getStatus)).allMatch(ConditionStatus.True::equals);
        }
    }

    void createOrbit() {
        operator()
                .getKubernetesClient()
                .resource(edit(new Orbit())
                        .with(metadata -> metadata.editMetadata()
                                .withName(operator().getNamespace())
                                .withLabels(Map.of(LABEL_K8S_COMPONENT, Constants.COMPONENT_ORBIT)))
                        .with(ResourceLoader::withResources)
                        .with(spec -> spec.editSpec()
                                .editConsensus()
                                .editGenesis()
                                .editAddressBook()
                                .withLedgerId("298"))
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

    void createCapsule(final Long nodeId) {
        createCapsule(nodeId, null);
    }

    void createCapsule(final Long nodeId, final SecretResources secretResources) {
        createCapsule(
                nodeId,
                Map.of(LABEL_K8S_MANAGED_BY, SOLO_OPERATOR, LABEL_K8S_COMPONENT, Constants.COMPONENT_CONSENSUS_NODE),
                secretResources);
    }

    void createCapsule(final Long nodeId, final Map<String, String> labels, final SecretResources secretResources) {
        String name = Metadata.prepareNodeName(nodeId);
        operator()
                .create(edit(new ConsensusCapsule())
                        .with(metadata -> metadata.editMetadata().withName(name).addToLabels(labels))
                        .with(ResourceLoader::withResources)
                        .with(spec -> spec.editSpec()
                                .withNodeId(nodeId)
                                .withAccountId("0.0.%d".formatted(nodeId))
                                .withOrbit(operator().getNamespace())
                                .editPodProperties()
                                .editContainers()
                                .editConsensusNode()
                                .editSoftwareVersion()
                                .withRepository("repository")
                                .withImageName("image")
                                .withImageTag("tag"))
                        .with(secret -> secret.editSpec().editSecrets().withSecretResources(secretResources))
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
