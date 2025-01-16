// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.sequencer;

import static org.assertj.core.api.AssertionsForClassTypes.assertThat;
import static org.junit.jupiter.api.Assertions.assertThrows;

import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationBuilder;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationSpec;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationSpecBuilder;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.ObjectMetaBuilder;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.fabric8.kubernetes.client.dsl.NamespaceableResource;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import org.junit.jupiter.api.Test;
import org.mockito.Mockito;

class SequencerTest {

    private final SequenceReconciler sequenceReconciler = new SequenceReconciler();
    private final Sequencer<NetworkOperation, NetworkOperationSpec, NetworkOperationStatus> sequencer =
            new Sequencer<>(sequenceReconciler);

    @Test
    void testLoad() {
        final String orbitName = "orbit-1";
        final NetworkOperation networkOperation1 = new NetworkOperationBuilder()
                .withMetadata(new ObjectMetaBuilder()
                        .withName("network-operation-1")
                        .withNamespace(orbitName)
                        .build())
                .withSpec(new NetworkOperationSpecBuilder().withOrbit(orbitName).build())
                .withStatus(new NetworkOperationStatus())
                .build();
        @SuppressWarnings("unchecked")
        Context<NetworkOperation> context = Mockito.mock(Context.class);
        KubernetesClient client = Mockito.mock(KubernetesClient.class);
        Mockito.when(context.getClient()).thenReturn(client);
        Mockito.when(client.resource(Mockito.any(ConfigMap.class))).thenAnswer(invocation -> {
            final ConfigMap cm = invocation.getArgument(0);
            @SuppressWarnings("unchecked")
            final NamespaceableResource<ConfigMap> nsr = Mockito.mock(NamespaceableResource.class);
            Mockito.when(nsr.get()).thenReturn(null);
            Mockito.when(nsr.create()).thenReturn(cm);
            Mockito.when(nsr.update()).thenReturn(cm);
            return nsr;
        });
        sequencer.load(networkOperation1, context, false);
        assertThat(sequencer.headSequenceNumber(networkOperation1)).isEqualTo(0L);
        assertThat(sequencer.tailSequenceNumber(networkOperation1)).isEqualTo(0L);

        networkOperation1.getMetadata().setNamespace("orbit-2");
        sequencer.load(networkOperation1, context, false); // reload should be ignored as it is loaded already
        // for a different orbit and resource, it would throw IllegalStateException
        assertThrows(IllegalStateException.class, () -> sequencer.headSequenceNumber(networkOperation1));
        assertThrows(IllegalStateException.class, () -> sequencer.tailSequenceNumber(networkOperation1));

        sequencer.load(networkOperation1, context, true); // force reload
        assertThat(sequencer.headSequenceNumber(networkOperation1)).isEqualTo(0L);
        assertThat(sequencer.tailSequenceNumber(networkOperation1)).isEqualTo(0L);
    }
}
