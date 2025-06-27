// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.sequencer;

import static com.hedera.solo.operator.library.sequencer.SequencerTestData.no1;
import static com.hedera.solo.operator.library.sequencer.SequencerTestData.no2;
import static com.hedera.solo.operator.library.sequencer.SequencerTestData.no3;
import static org.assertj.core.api.Assertions.assertThat;

import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import io.fabric8.kubernetes.api.model.KubernetesResourceList;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.fabric8.kubernetes.client.dsl.MixedOperation;
import io.fabric8.kubernetes.client.dsl.Resource;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.DeleteControl;
import io.javaoperatorsdk.operator.api.reconciler.UpdateControl;
import java.util.List;
import org.junit.jupiter.api.Test;
import org.mockito.Mockito;

/**
 * The SequenceReconcilerTest class is a JUnit test case for the SequenceReconcilerMock class. It tests the reconcile
 * method to ensure that network operations are processed correctly.
 */
class SequenceReconcilerTest {
    @Test
    void testReconcile() throws Exception {
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

        @SuppressWarnings("unchecked")
        final Context<NetworkOperation> context = Mockito.mock(Context.class);
        final SequenceReconcilerMock sequenceReconciler = new SequenceReconcilerMock(client);

        // reconcile the network operations
        UpdateControl<NetworkOperation> resp = sequenceReconciler.reconcile(no1, context);
        assertThat(resp.isPatchStatus()).isTrue();
        assertThat(resp.getResource()).isPresent();
        assertThat(resp.getResource().orElseThrow().getStatus().getPhase())
                .isEqualTo(NetworkOperationStatus.Phase.Succeeded);

        // try to process a resource that is not the first in the queue
        resp = sequenceReconciler.reconcile(no3, context);
        assertThat(resp.isPatchStatus()).isFalse();
        assertThat(resp.getScheduleDelay()).isPresent();

        // after delete, reattempt would be idempotent and return no update
        DeleteControl resp3 = sequenceReconciler.cleanup(no3, context);
        assertThat(resp3.isRemoveFinalizer()).isTrue();
        resp = sequenceReconciler.reconcile(no3, context);
        assertThat(resp.isNoUpdate()).isTrue();
    }
}
