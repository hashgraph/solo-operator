// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.operator.library.sequencer.SequencerTestData.no1;
import static com.hedera.solo.operator.library.sequencer.SequencerTestData.no2;
import static com.hedera.solo.operator.library.sequencer.SequencerTestData.no3;
import static org.assertj.core.api.AssertionsForClassTypes.assertThat;

import com.hedera.solo.operator.library.k8s.CustomResourceManager;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsuleBuilder;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.reconciler.networkoperation.check.NetworkCheckProcessor;
import com.hedera.solo.operator.reconciler.networkoperation.genesis.NetworkGenesisProcessor;
import com.hedera.solo.operator.reconciler.networkoperation.node.NodeOperationProcessor;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.NetworkUpgradeProcessor;
import io.fabric8.kubernetes.api.model.KubernetesResourceList;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.fabric8.kubernetes.client.dsl.MixedOperation;
import io.fabric8.kubernetes.client.dsl.Resource;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.DeleteControl;
import io.javaoperatorsdk.operator.api.reconciler.UpdateControl;
import java.util.List;
import org.assertj.core.api.Assertions;
import org.junit.jupiter.api.Test;
import org.mockito.Mockito;

class NetworkOperationSequenceReconcilerTest {

    private static KubernetesClient mockKubernetesClient() {
        final KubernetesClient client = Mockito.mock(KubernetesClient.class);
        Mockito.when(client.getNamespace()).thenReturn("test");

        // return a list of network operations
        Mockito.when(client.resources(NetworkOperation.class)).thenAnswer(invocation -> {
            @SuppressWarnings("unchecked")
            final MixedOperation<NetworkOperation, KubernetesResourceList<NetworkOperation>, Resource<NetworkOperation>>
                    mixedOperation = Mockito.mock(MixedOperation.class);
            @SuppressWarnings("unchecked")
            final KubernetesResourceList<NetworkOperation> list = Mockito.mock(KubernetesResourceList.class);
            Mockito.when(mixedOperation.list()).thenReturn(list);
            Mockito.when(list.getItems()).thenReturn(List.of(no1, no2, no3));
            return mixedOperation;
        });

        return client;
    }

    private NetworkOperationSequenceReconciler prepareReconciler() {
        final String namespace = no1.getMetadata().getNamespace();
        final CustomResourceManager customResourceManager = Mockito.mock(CustomResourceManager.class);

        Mockito.when(customResourceManager.consensusCapsules(namespace))
                .thenReturn(List.of(
                        new ConsensusCapsuleBuilder()
                                .withNewMetadata()
                                .withName("node0")
                                .withNamespace(namespace)
                                .endMetadata()
                                .build(),
                        new ConsensusCapsuleBuilder()
                                .withNewMetadata()
                                .withName("node1")
                                .withNamespace(namespace)
                                .endMetadata()
                                .build()));

        Mockito.when(customResourceManager.networkOperations()).thenReturn(List.of(no1, no2, no3));

        final NetworkGenesisProcessor networkGenesisProcessor = Mockito.mock(NetworkGenesisProcessor.class);
        final NetworkUpgradeProcessor networkUpgradeProcessor = Mockito.mock(NetworkUpgradeProcessor.class);
        final NetworkCheckProcessor networkCheckProcessor = Mockito.mock(NetworkCheckProcessor.class);
        final NodeOperationProcessor nodeOperationProcessor = Mockito.mock(NodeOperationProcessor.class);

        return new NetworkOperationSequenceReconciler(
                customResourceManager,
                networkGenesisProcessor,
                networkUpgradeProcessor,
                networkCheckProcessor,
                nodeOperationProcessor);
    }

    @Test
    void reconcile() throws Exception {
        final NetworkOperationSequenceReconciler reconciler = prepareReconciler();
        @SuppressWarnings("unchecked")
        final Context<NetworkOperation> context = Mockito.mock(Context.class);

        // different version shouldn't be processed
        NetworkOperation no1v1 =
                edit(no1).with(r -> r.editSpec().withVersion("1")).build();
        UpdateControl<NetworkOperation> resp = reconciler.reconcile(no1v1, context);
        Assertions.assertThat(resp.isNoUpdate()).isTrue();

        // reconcile the network operations
        resp = reconciler.reconcile(no1, context);
        Assertions.assertThat(resp.isPatchStatus()).isTrue();
        Assertions.assertThat(resp.getResource().getStatus().getPhase())
                .isEqualTo(NetworkOperationStatus.Phase.Pending);

        // try to process a resource that is not the first in the queue
        resp = reconciler.reconcile(no3, context);
        Assertions.assertThat(resp.isPatchStatus()).isTrue();
        Assertions.assertThat(resp.getResource().getStatus().getPhase())
                .isEqualTo(NetworkOperationStatus.Phase.Pending);
    }

    @Test
    void activeResources() {
        final NetworkOperationSequenceReconciler reconciler = prepareReconciler();
        List<NetworkOperation> activeResources = NetworkOperationSequenceReconciler.activeResources();
        assertThat(activeResources).isNotNull();
        assertThat(activeResources.size()).isEqualTo(3);
    }

    @Test
    void cleanup() {
        final NetworkOperationSequenceReconciler reconciler = prepareReconciler();
        @SuppressWarnings("unchecked")
        final Context<NetworkOperation> context = Mockito.mock(Context.class);
        DeleteControl resp = reconciler.cleanup(no1, context);
        assertThat(resp).isNotNull();
        assertThat(resp.isRemoveFinalizer()).isTrue();
    }
}
