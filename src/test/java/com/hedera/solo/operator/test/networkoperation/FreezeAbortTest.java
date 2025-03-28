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
        Context context = mock(Context.class);

        // test with empty status
        NetworkOperationStatus status = new NetworkOperationStatus();
        status = step.run(resource, status, context);
        assertThat(status).isNotNull();
        assertThat(status.getPhase()).isEqualTo(NetworkOperationStatus.Phase.Succeeded);
        assertThat(status.getConditions().size()).isEqualTo(1);
        assertThat(status.getConditions()).anyMatch(c -> FreezeAbortStep.CONDITION_TYPE.equals(c.getType()));
        assertThat(status.getConditions())
                .anyMatch(c -> resource.getMetadata().getGeneration().equals(c.getObservedGeneration()));

        // test with existing status
        resource.getMetadata().setGeneration(1L);
        status = step.run(resource, status, context);
        assertThat(status).isNotNull();
        assertThat(status.getPhase()).isEqualTo(NetworkOperationStatus.Phase.Succeeded);
        assertThat(status.getConditions().size()).isEqualTo(1);
        assertThat(status.getConditions()).anyMatch(c -> FreezeAbortStep.CONDITION_TYPE.equals(c.getType()));
        assertThat(status.getConditions())
                .anyMatch(c -> resource.getMetadata().getGeneration().equals(c.getObservedGeneration()));
    }
}
