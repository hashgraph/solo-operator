// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.test.networkoperation;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.operator.model.v1alpha1.NetworkOperationSpec.Mode.FirstInitialization;
import static com.hedera.solo.operator.reconciler.common.Constants.OPERATOR_PREFIX;
import static java.util.Collections.emptyMap;
import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.reconciler.common.ConditionStatus;
import com.hedera.solo.operator.reconciler.common.Metadata;
import com.hedera.solo.operator.test.fixtures.annotations.SoloTest;
import com.hedera.solo.operator.test.fixtures.resource.LoadResource;
import com.hedera.solo.operator.test.fixtures.snapshot.SnapshotFailureContext;
import com.hedera.solo.test.framework.AbstractOperatorTest;
import io.fabric8.kubernetes.api.model.Condition;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.javaoperatorsdk.operator.api.config.ControllerConfigurationOverrider;
import io.javaoperatorsdk.operator.junit.LocallyRunOperatorExtension;
import java.util.ArrayList;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.concurrent.TimeUnit;
import java.util.stream.Collectors;
import org.apache.commons.lang3.StringUtils;
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
        verifyClusterConfig(capsuleNames);
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

        verifyClusterConfig(capsuleNames);
    }

    @Test
    void failNetworkOperation() {
        List<Long> capsuleNodeIds = prepareNodeIds(3);
        capsuleNodeIds.forEach(nodeId -> createCapsule(nodeId, Map.of("fail", "true")));

        createNetworkOperation("init", false);
        await().atMost(30, TimeUnit.SECONDS).untilAsserted(() -> {
            NetworkOperation no = operator().get(NetworkOperation.class, "init");
            assertThat(no).isNotNull();
            assertThat(no.getStatus()).isNotNull();
            assertThat(no.getStatus().getPhase()).isEqualTo(NetworkOperationStatus.Phase.Failed);
            assertThat(no.getStatus().getConditions().size()).isEqualTo(1);
            Condition condition = no.getStatus().getConditions().get(0);
            assertThat(condition.getStatus()).isEqualTo(ConditionStatus.False);
            assertThat(StringUtils.countMatches(condition.getMessage(), "Failed"))
                    .isEqualTo(capsuleNodeIds.size());
        });
    }

    void createNetworkOperation(final String name, final boolean noUpdate) {
        final String orbitName = operator().getNamespace(); // for now namespace and orbit name are the same
        operator()
                .create(edit(new NetworkOperation())
                        .with(metadata -> metadata.editMetadata()
                                .withName(name)
                                .addToLabels("no-update", String.valueOf(noUpdate)))
                        .with(spec ->
                                spec.editSpec().withMode(FirstInitialization).withOrbit(orbitName))
                        .build());
    }

    void verifyClusterConfig(final List<String> capsuleNames) {
        ConfigMap cm = operator().get(ConfigMap.class, "cluster-config");
        assertThat(cm).isNotNull();
        String configTxt = cm.getData().get("config.txt");
        assertThat(configTxt).isNotNull();
        List<String> lines =
                configTxt.lines().filter(line -> line.startsWith("address")).toList();
        assertThat(lines).size().isEqualTo(3);
        Map<String, Integer> nodes = lines.stream()
                .map(line -> line.split("\\s*,\\s*"))
                .collect(Collectors.toMap(a -> a[2], a -> Integer.valueOf(a[1])));
        assertThat(nodes.keySet()).isEqualTo(new HashSet<>(capsuleNames));
        assertThat(nodes.values().stream().distinct().count()).isEqualTo(capsuleNames.size());
        assertThat(nodes.values().stream().min(Integer::compareTo)).contains(0);
        assertThat(nodes.values().stream().max(Integer::compareTo)).contains(capsuleNames.size() - 1);
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
            assertThat(no.getStatus().getConditions().size()).isEqualTo(1);
            Condition condition = no.getStatus().getConditions().get(0);
            assertThat(condition.getStatus()).isEqualTo(ConditionStatus.True);
        }
    }

    void createOrbit() {
        operator()
                .getKubernetesClient()
                .resource(edit(new Orbit())
                        .with(metadata ->
                                metadata.editMetadata().withName(operator().getNamespace()))
                        .with(LoadResource::orbitBuilderWithResources)
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
        createCapsule(nodeId, emptyMap());
    }

    void createCapsule(final Long nodeId, final Map<String, String> labels) {
        String name = Metadata.prepareNodeName(nodeId);
        operator()
                .create(edit(new ConsensusCapsule())
                        .with(metadata -> metadata.editMetadata().withName(name).addToLabels(labels))
                        .with(LoadResource::consensusCapsuleBuilderWithResources)
                        .with(spec -> spec.editSpec()
                                .withNodeId(nodeId)
                                .withAccountId("0.0.%d".formatted(nodeId))
                                .withOrbit(operator().getNamespace()))
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
