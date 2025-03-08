// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.test.networkoperation.upgrade.render;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.test.framework.KubernetesDevServiceExecutionListener.initialize;
import static com.hedera.solo.test.networkoperation.upgrade.render.SystemTestUtils.create;
import static com.hedera.solo.test.networkoperation.upgrade.render.SystemTestUtils.createNamespace;
import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

import com.hedera.solo.operator.model.commonspec.AddressBookEntry;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsuleSpec;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.ApplyConsensusPodChangeStep;
import com.hedera.solo.operator.test.fixtures.fuzz.BeanFactory;
import io.fabric8.kubernetes.client.CustomResource;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.junit.AbstractOperatorExtension;
import io.javaoperatorsdk.operator.junit.LocallyRunOperatorExtension;
import java.util.Collections;
import java.util.HashSet;
import java.util.List;
import java.util.Set;
import java.util.stream.Collectors;
import java.util.stream.Stream;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.RegisterExtension;

public class NowFrozenConsensusCapsulesSystemTest extends NowFrozenConsensusCapsulesUnitTest {
    String namespaceInitial;
    String namespace;
    KubernetesClient kubernetesClient;

    @RegisterExtension
    private AbstractOperatorExtension operator = initialize(this::configure);

    protected LocallyRunOperatorExtension.Builder configure(final KubernetesClient client) {
        return LocallyRunOperatorExtension.builder()
                .withKubernetesClient(client)
                .withReconciler(NoOpConsensusCapsuleReconciler.class)
                .withReconciler(NoOpNetworkOperationReconciler.class);
    }

    @Override
    BeanFactory beanFactory() {
        return new SystemTestBeanFactory();
    }

    @BeforeEach
    public void setup() {
        kubernetesClient = operator.getKubernetesClient();
        namespaceInitial = createNamespace(kubernetesClient, getClass());
        namespace = createNamespace(kubernetesClient, getClass());
    }

    @AfterEach
    public void cleanup() {
        kubernetesClient.namespaces().withName(namespace).delete();
        kubernetesClient.namespaces().withName(namespaceInitial).delete();
    }

    @Test
    @Override
    public void test() {
        super.test();
        NetworkOperation networkOperation = kubernetesClient
                .resources(NetworkOperation.class)
                .inNamespace(namespace)
                .list()
                .getItems()
                .getFirst();
        Set<Long> capsules =
                kubernetesClient.resources(ConsensusCapsule.class).inNamespace(namespace).list().getItems().stream()
                        .map(CustomResource::getSpec)
                        .map(ConsensusCapsuleSpec::getNodeId)
                        .collect(Collectors.toSet());
        Set<Long> addressBook = networkOperation.getSpec().getNetworkUpgrade().getAddressBook().stream()
                .map(AddressBookEntry::getNodeId)
                .collect(Collectors.toSet());
        Set<Long> upgraded = new HashSet<>(networkOperation.getStatus().getUpgradedNodes());
        Set<Long> intersection = new HashSet<>(addressBook);
        intersection.retainAll(capsules);
        assertThat(upgraded).isEqualTo(intersection);
    }

    @Override
    NetworkOperation networkOperation(final Set<Long> nodeIds) {
        return create(
                kubernetesClient,
                edit(super.networkOperation(nodeIds))
                        .with(metadata -> metadata.editMetadata()
                                .withName(null)
                                .withGenerateName("no-")
                                .withNamespace(namespace))
                        .with(status -> status.editStatus().withUpgradedNodes(Collections.emptyList()))
                        .build());
    }

    @Override
    ConsensusCapsule consensusCapsule(final NetworkOperation networkOperation, final long nodeId) {
        ConsensusCapsule capsule = edit(super.consensusCapsule(networkOperation, nodeId))
                .with(metadata -> metadata.editMetadata()
                        .withName(null)
                        .withGenerateName("cc-")
                        .withNamespace(namespaceInitial))
                .build();
        capsule.getMetadata()
                .setName(create(kubernetesClient, capsule).getMetadata().getName());
        capsule.getMetadata().setNamespace(namespace);
        return create(kubernetesClient, capsule);
    }

    @Override
    Stream<Update> update(final NetworkOperation networkOperation, final Stream<ConsensusCapsule> capsules) {
        @SuppressWarnings("unchecked")
        Context<NetworkOperation> context = mock(Context.class);
        when(context.getClient()).thenReturn(kubernetesClient);
        ApplyConsensusPodChangeStep applyConsensusPodChangeStep = new ApplyConsensusPodChangeStep();
        List<String> names = capsules.map(c -> c.getMetadata().getName()).toList();
        NetworkOperationStatus updatedStatus =
                applyConsensusPodChangeStep.run(networkOperation, networkOperation.getStatus(), context);
        kubernetesClient
                .resource(networkOperation.edit().withStatus(updatedStatus).build())
                .patchStatus();
        return names.stream()
                .map(name -> new Update(
                        kubernetesClient
                                .resources(ConsensusCapsule.class)
                                .inNamespace(namespaceInitial)
                                .withName(name)
                                .get(),
                        kubernetesClient
                                .resources(ConsensusCapsule.class)
                                .inNamespace(namespace)
                                .withName(name)
                                .get()));
    }
}
