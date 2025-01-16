// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.sequencer;

import static org.assertj.core.api.AssertionsForClassTypes.assertThat;

import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationBuilder;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationSpecBuilder;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.ObjectMetaBuilder;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.fabric8.kubernetes.client.dsl.NamespaceableResource;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.UpdateControl;
import java.time.Duration;
import org.junit.jupiter.api.Test;
import org.mockito.Mockito;

public class SequenceReconcilerTest {
    private final SequenceReconciler sequenceReconciler = new SequenceReconciler();

    @Test
    void testComputeReconcileDelay() {
        NetworkOperation networkOperation = new NetworkOperation();
        assertThat(sequenceReconciler.computeReconcileDelay(networkOperation)).isEqualTo(Duration.ofSeconds(1));
    }

    @Test
    void testReconcile() throws Exception {
        final String orbitName = "orbit-1";
        final NetworkOperation networkOperation1 = new NetworkOperationBuilder()
                .withMetadata(new ObjectMetaBuilder()
                        .withName("network-operation-1")
                        .withNamespace(orbitName)
                        .build())
                .withSpec(new NetworkOperationSpecBuilder().withOrbit(orbitName).build())
                .withStatus(new NetworkOperationStatus())
                .build();
        final NetworkOperation networkOperation2 = new NetworkOperationBuilder()
                .withMetadata(new ObjectMetaBuilder()
                        .withName("network-operation-2")
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

        UpdateControl<NetworkOperation> resp1 = sequenceReconciler.reconcile(networkOperation1, context);
        UpdateControl<NetworkOperation> resp2 = sequenceReconciler.reconcile(networkOperation2, context);
        final Long headSeq = sequenceReconciler.sequencer.headSequenceNumber(
                resp1.getResource().getMetadata().getNamespace());
        final Long tailSeq = sequenceReconciler.sequencer.tailSequenceNumber(
                resp1.getResource().getMetadata().getNamespace());
        assertThat(headSeq).isEqualTo(0);
        assertThat(tailSeq).isEqualTo(2);
        assertThat(resp1).isNotNull();
        assertThat(resp1.isPatchStatus()).isTrue();
        assertThat(resp1.getResource().getStatus()).isNotNull();
        assertThat(resp1.getResource().getStatus().getConditions()).isNull();
        assertThat(resp1.getResource().getStatus().getPhase()).isEqualTo(NetworkOperationStatus.Phase.Pending);
        assertThat(sequenceReconciler.getSequenceNumber(resp1.getResource())).isEqualTo(headSeq);

        assertThat(resp2).isNotNull();
        assertThat(resp2.isPatchStatus()).isTrue();
        assertThat(resp2.getResource().getStatus().getConditions()).isNull();
        assertThat(resp2.getResource().getStatus().getPhase()).isEqualTo(NetworkOperationStatus.Phase.Pending);
        assertThat(sequenceReconciler.getSequenceNumber(resp2.getResource())).isEqualTo(headSeq + 1);

        // second in queue resource shouldn't have new sequence number if reconciled again
        UpdateControl<NetworkOperation> resp22 = sequenceReconciler.reconcile(resp2.getResource(), context);
        assertThat(resp22).isNotNull();
        assertThat(resp22.isNoUpdate()).isTrue();
        assertThat(resp22.isPatchStatus()).isFalse();

        // fist in queue resource should be processed if reconciled again
        UpdateControl<NetworkOperation> resp12 = sequenceReconciler.reconcile(resp1.getResource(), context);
        assertThat(resp12).isNotNull();
        assertThat(resp12.isPatchStatus()).isTrue();
        assertThat(sequenceReconciler.getSequenceNumber(resp12.getResource())).isEqualTo(headSeq);
        assertThat(resp12.getResource().getStatus().getConditions()).isNotNull();
        assertThat(resp12.getResource().getStatus().getConditions().isEmpty()).isFalse(); // a condition is added
        assertThat(resp12.getResource().getStatus().getPhase()).isEqualTo(NetworkOperationStatus.Phase.Succeeded);

        // head sequence number should be updated, tail should remain the same
        assertThat(sequenceReconciler.sequencer.headSequenceNumber(
                        resp12.getResource().getMetadata().getNamespace()))
                .isEqualTo(1);
        assertThat(sequenceReconciler.sequencer.tailSequenceNumber(
                        resp12.getResource().getMetadata().getNamespace()))
                .isEqualTo(tailSeq);
    }
}
