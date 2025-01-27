// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation;

import static org.assertj.core.api.AssertionsForClassTypes.assertThat;

import com.hedera.solo.operator.library.processor.ProcessorResult;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationBuilder;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import java.time.Duration;
import org.junit.jupiter.api.Test;
import org.mockito.Mockito;

class NetworkOperationReconcilerV2Test {

    @Test
    void getName() {
        final NetworkOperationReconcilerV2 networkOperationReconcilerV2 = new NetworkOperationReconcilerV2();
        assertThat(networkOperationReconcilerV2.getName())
                .isEqualTo(NetworkOperationReconcilerV2.class.getName().toLowerCase());
    }

    @Test
    void getOrbitName() {
        final NetworkOperationReconcilerV2 networkOperationReconcilerV2 = new NetworkOperationReconcilerV2();
        final NetworkOperation networkOperation = new NetworkOperationBuilder()
                .withNewSpec()
                .withOrbit("orbit-1")
                .endSpec()
                .build();
        assertThat(networkOperationReconcilerV2.getOrbitName(networkOperation))
                .isEqualTo(networkOperation.getSpec().getOrbit());
    }

    @Test
    void getSequenceNumber() {
        final NetworkOperationReconcilerV2 networkOperationReconcilerV2 = new NetworkOperationReconcilerV2();
        final NetworkOperation networkOperation = new NetworkOperationBuilder()
                .withNewStatus()
                .withSequenceNumber(1L)
                .endStatus()
                .build();
        assertThat(networkOperationReconcilerV2.getSequenceNumber(networkOperation))
                .isEqualTo(networkOperation.getStatus().getSequenceNumber());
    }

    @Test
    void setSequenceNumber() {
        final NetworkOperationReconcilerV2 networkOperationReconcilerV2 = new NetworkOperationReconcilerV2();
        final NetworkOperation networkOperation = new NetworkOperationBuilder()
                .withNewStatus()
                .withSequenceNumber(1L)
                .endStatus()
                .build();

        @SuppressWarnings("unchecked")
        final Context<NetworkOperation> context = Mockito.mock(Context.class);
        final NetworkOperation resp = networkOperationReconcilerV2.setSequenceNumber(networkOperation, context, 2L);
        assertThat(networkOperation.getStatus().getSequenceNumber()).isEqualTo(1L);
        assertThat(resp.getStatus().getSequenceNumber()).isEqualTo(2L);
        assertThat(resp.getStatus().getPhase()).isEqualTo(NetworkOperationStatus.Phase.Pending);
    }

    @Test
    void shouldProcess() {
        final NetworkOperationReconcilerV2 networkOperationReconcilerV2 = new NetworkOperationReconcilerV2();
        final NetworkOperation networkOperation = new NetworkOperationBuilder()
                .withNewSpec()
                .endSpec()
                .withNewStatus()
                .endStatus()
                .build();
        @SuppressWarnings("unchecked")
        final Context<NetworkOperation> context = Mockito.mock(Context.class);
        assertThat(networkOperationReconcilerV2.shouldProcess(networkOperation, context))
                .isFalse();

        final NetworkOperation networkOperation2 = new NetworkOperationBuilder()
                .withNewSpec()
                .withVersion("v2")
                .endSpec()
                .withNewStatus()
                .endStatus()
                .build();
        assertThat(networkOperationReconcilerV2.shouldProcess(networkOperation2, context))
                .isTrue();
    }

    @Test
    void computeReconcileDelay() {
        final NetworkOperationReconcilerV2 networkOperationReconcilerV2 = new NetworkOperationReconcilerV2();
        final NetworkOperation networkOperation = new NetworkOperationBuilder()
                .withNewSpec()
                .endSpec()
                .withNewStatus()
                .endStatus()
                .build();
        assertThat(networkOperationReconcilerV2.computeReconcileDelay(networkOperation, null))
                .isEqualTo(Duration.ofSeconds(1));

        @SuppressWarnings("unchecked")
        final ProcessorResult<NetworkOperationStatus> result = Mockito.mock(ProcessorResult.class);
        Mockito.when(result.getReconcileDelay()).thenReturn(Duration.ofSeconds(2));
        assertThat(networkOperationReconcilerV2.computeReconcileDelay(networkOperation, result))
                .isEqualTo(Duration.ofSeconds(2));
    }
}
