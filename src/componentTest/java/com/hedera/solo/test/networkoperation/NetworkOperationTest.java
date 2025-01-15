// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.test.networkoperation;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.operator.model.v1alpha1.NetworkOperationSpec.Mode.FirstInitialization;
import static com.hedera.solo.operator.reconciler.common.Constants.OPERATOR_PREFIX;
import static java.util.Collections.emptyMap;
import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

import com.hedera.solo.operator.model.commonspec.KeyReference;
import com.hedera.solo.operator.model.commonspec.KeyReferenceBuilder;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.secrets.SecretResources;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.secrets.SecretResourcesBuilder;
import com.hedera.solo.operator.reconciler.common.ConditionStatus;
import com.hedera.solo.operator.reconciler.common.Metadata;
import com.hedera.solo.operator.test.fixtures.annotations.SoloTest;
import com.hedera.solo.operator.test.fixtures.resource.ResourceLoader;
import com.hedera.solo.operator.test.fixtures.snapshot.SnapshotFailureContext;
import com.hedera.solo.test.framework.AbstractOperatorTest;
import io.fabric8.kubernetes.api.model.Condition;
import io.fabric8.kubernetes.api.model.Event;
import io.fabric8.kubernetes.api.model.ObjectReference;
import io.fabric8.kubernetes.api.model.Secret;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.javaoperatorsdk.operator.api.config.ControllerConfigurationOverrider;
import io.javaoperatorsdk.operator.junit.LocallyRunOperatorExtension;
import java.util.ArrayList;
import java.util.Collection;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.concurrent.TimeUnit;
import java.util.stream.Collectors;
import java.util.stream.LongStream;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

@SoloTest
public class NetworkOperationTest extends AbstractOperatorTest {
    @Override
    protected LocallyRunOperatorExtension.Builder configure(final KubernetesClient client) {
        return builder(client)
                .withReconciler(
                        new NetworkOperationTestReconciler(), ControllerConfigurationOverrider::watchingAllNamespaces)
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

    @Test
    void oneNetworkOperationAtATime() {
        int total = 20;
        List<Long> capsuleNodeIds = prepareNodeIds(3);
        List<String> capsuleNames = prepareNodeNames(capsuleNodeIds);

        capsuleNodeIds.forEach(this::createCapsule);

        for (int i = 0; i < total; ++i) {
            String name = "init-" + i;
            createNetworkOperation(name, false);
            await().atMost(30, TimeUnit.SECONDS).untilAsserted(() -> {
                verifyNetworkOperationSucceeded(name, true);
            });
        }
    }

    @Test
    void manyNetworkOperationsAtOnceWithDeletions() {
        int total = 20;
        int noUpdatePeriod = 7;
        List<Long> capsuleNodeIds = prepareNodeIds(3);
        List<String> capsuleNames = prepareNodeNames(capsuleNodeIds);

        capsuleNodeIds.forEach(this::createCapsule);

        for (int i = 0; i < total; ++i) {
            createNetworkOperation("init-" + i, 0 == i % noUpdatePeriod);
        }

        for (int i = 0; i < total; i += noUpdatePeriod) {
            deleteNetworkOperationStuckInPending("init-" + i);
        }

        await().atMost(30, TimeUnit.SECONDS).untilAsserted(() -> {
            for (int i = 0; i < total; ++i) {
                verifyNetworkOperationSucceeded("init-" + i, 0 != i % noUpdatePeriod);
            }
        });
    }

    @Test
    void failNetworkOperation() {
        List<Long> capsuleNodeIds = prepareNodeIds(3);
        capsuleNodeIds.forEach(nodeId -> createCapsule(nodeId, Map.of("fail", "true"), null));

        createNetworkOperation("init", false);
        await().atMost(30, TimeUnit.SECONDS).untilAsserted(() -> {
            NetworkOperation no = operator().get(NetworkOperation.class, "init");
            assertThat(no).isNotNull();
            assertThat(no.getStatus()).isNotNull();
            assertThat(no.getStatus().getPhase()).isEqualTo(NetworkOperationStatus.Phase.Failed);
            Map<String, List<Condition>> conditions =
                    no.getStatus().getConditions().stream().collect(Collectors.groupingBy(Condition::getType));
            assertThat(conditions.values().stream().flatMap(Collection::stream))
                    .size()
                    .isEqualTo(2);
            Condition ready = conditions.get("Ready").getFirst();
            Condition valid = conditions.get("ManifestValidation").getFirst();
            assertThat(ready.getStatus()).isEqualTo(ConditionStatus.False);
            assertThat(ready.getMessage()).isEqualTo("Capsule phases: {node1=Failed, node2=Failed, node3=Failed}");
            assertThat(valid.getStatus()).isEqualTo(ConditionStatus.True);
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
                        .with(metadata -> metadata.editMetadata().withGenerateName("secret-"))
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
        LongStream.range(0, 4).forEach(id -> createCapsule(id, emptyMap(), secretResources));

        String name = "init";
        NetworkOperation networkOperation = createNetworkOperation(name, false);
        await().atMost(30, TimeUnit.SECONDS).untilAsserted(() -> {
            NetworkOperation no = operator().get(NetworkOperation.class, name);
            assertThat(no).isNotNull();
            assertThat(no.getStatus()).isNotNull();
            assertThat(no.getStatus().getConditions()).isNotNull();
            List<Condition> conditions = no.getStatus().getConditions().stream()
                    .filter(c -> "ManifestValidation".equals(c.getType()))
                    .toList();
            assertThat(conditions.size()).isEqualTo(1);
            Condition condition = conditions.getFirst();
            assertThat(condition.getStatus()).isEqualTo(valid ? ConditionStatus.True : ConditionStatus.False);
            if (!valid) {
                assertThat(condition.getReason()).isEqualTo("SecretsValid");
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
            assertThat(event.getMessage()).contains("Exception");
        });
    }

    NetworkOperation createNetworkOperation(final String name, final boolean noUpdate) {
        final String orbitName = operator().getNamespace(); // for now namespace and orbit name are the same
        return operator()
                .create(edit(new NetworkOperation())
                        .with(metadata -> metadata.editMetadata()
                                .withName(name)
                                .addToLabels("no-update", String.valueOf(noUpdate)))
                        .with(spec ->
                                spec.editSpec().withMode(FirstInitialization).withOrbit(orbitName))
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
            assertThat(conditions.size()).isEqualTo(2);
            assertThat(conditions.stream().map(Condition::getType).collect(Collectors.toSet()))
                    .isEqualTo(Set.of("ManifestValidation", "Ready"));
            assertThat(conditions.stream().map(Condition::getStatus)).allMatch(ConditionStatus.True::equals);
        }
    }

    void createOrbit() {
        operator()
                .getKubernetesClient()
                .resource(edit(new Orbit())
                        .with(metadata ->
                                metadata.editMetadata().withName(operator().getNamespace()))
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
        createCapsule(nodeId, emptyMap(), null);
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
