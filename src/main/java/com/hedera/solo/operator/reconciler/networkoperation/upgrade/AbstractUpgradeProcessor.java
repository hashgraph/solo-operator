// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;

import com.hedera.solo.operator.library.processor.AbstractRollbackProcessor;
import com.hedera.solo.operator.library.processor.ProcessorException;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.reconciler.common.ConditionStatus;
import com.hedera.solo.operator.reconciler.common.Constants;
import io.fabric8.kubernetes.api.model.Condition;
import io.fabric8.kubernetes.api.model.ConditionBuilder;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import java.time.Instant;
import java.util.ArrayList;
import java.util.List;

public abstract class AbstractUpgradeProcessor extends AbstractRollbackProcessor<NetworkOperation> {
    protected Condition prepareSuccessCondition() {
        final String name = getProcessorName();
        return new ConditionBuilder()
                .withType(Constants.CONDITION_TYPE_READY)
                .withStatus(ConditionStatus.True)
                .withReason(name)
                .withMessage(SUCCESS_MESSAGE.formatted(name))
                .withLastTransitionTime(Instant.now().toString())
                .build();
    }

    protected Condition prepareFailureCondition(List<ProcessorException> exceptions) {
        final String name = getProcessorName();
        final List<String> logs = new ArrayList<>();
        for (ProcessorException e : exceptions) {
            logs.add(e.getMessage());
        }

        return new ConditionBuilder()
                .withType(Constants.CONDITION_TYPE_READY)
                .withStatus(ConditionStatus.False)
                .withReason(name)
                .withMessage(FAILURE_MESSAGE.formatted(name, String.join(",", logs)))
                .withLastTransitionTime(Instant.now().toString())
                .build();
    }

    @Override
    protected NetworkOperation addSuccessCondition(NetworkOperation primary, Context<NetworkOperation> context) {
        final Condition condition = this.prepareSuccessCondition();
        return edit(primary)
                .with(status -> status.editStatus().addToConditions(condition))
                .build();
    }

    @Override
    protected NetworkOperation addFailureCondition(
            NetworkOperation primary, Context<NetworkOperation> context, List<ProcessorException> exceptions) {

        final Condition condition = this.prepareFailureCondition(exceptions);
        return edit(primary)
                .with(status -> status.editStatus().addToConditions(condition))
                .build();
    }
}
