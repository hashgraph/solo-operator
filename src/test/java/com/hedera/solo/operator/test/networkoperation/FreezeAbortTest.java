// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.networkoperation;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.mock;

import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationSpec;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.model.v1alpha1.networkoperationspec.NetworkUpgrade;
import com.hedera.solo.operator.model.v1alpha1.networkoperationspec.NetworkUpgradeBuilder;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.FreezeAbortStep;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import org.junit.jupiter.api.Test;

/**
 * This class contains tests for the FreezeAbortStep implementation. The FreezeAbortStep is responsible for handling the
 * freeze abort workflow step in a network operation.
 */
public class FreezeAbortTest {
    @Test
    public void test() {
        NetworkOperation resource = edit(new NetworkOperation())
                .with(metadata -> metadata.editMetadata()
                        .withNamespace("test")
                        .withName("network-upgrade")
                        .withGeneration(0L))
                .with(spec -> spec.editSpec()
                        .withMode(NetworkOperationSpec.Mode.NetworkUpgrade)
                        .withNetworkUpgrade(new NetworkUpgradeBuilder()
                                .withWorkflowStep(NetworkUpgrade.WorkflowStep.FreezeAbort)
                                .build()))
                .build();
        FreezeAbortStep step = new FreezeAbortStep();

        @SuppressWarnings("unchecked")
        Context<NetworkOperation> context = mock(Context.class);

        // Test with an empty status.
        NetworkOperationStatus status = new NetworkOperationStatus();
        status = step.run(resource, status, context);
        assertThat(status).isNotNull();
        assertThat(status.getPhase()).isEqualTo(NetworkOperationStatus.Phase.Succeeded);
        assertThat(status.getConditions().size()).isEqualTo(1);
        assertThat(status.getConditions())
                .anyMatch(c -> FreezeAbortStep.CONDITION_TYPE.type().equals(c.getType())
                        && FreezeAbortStep.CONDITION_TYPE.reason().equals(c.getReason()));
        assertThat(status.getConditions())
                .anyMatch(c -> resource.getMetadata().getGeneration().equals(c.getObservedGeneration()));

        // Test with an existing status and updated resource generation.
        resource.getMetadata().setGeneration(1L);
        status = step.run(resource, status, context);
        assertThat(status).isNotNull();
        assertThat(status.getPhase()).isEqualTo(NetworkOperationStatus.Phase.Succeeded);
        assertThat(status.getConditions().size()).isEqualTo(1);
        assertThat(status.getConditions())
                .anyMatch(c -> FreezeAbortStep.CONDITION_TYPE.type().equals(c.getType())
                        && FreezeAbortStep.CONDITION_TYPE.reason().equals(c.getReason()));
        assertThat(status.getConditions())
                .anyMatch(c -> resource.getMetadata().getGeneration().equals(c.getObservedGeneration()));
    }
}
