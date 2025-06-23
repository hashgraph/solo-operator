// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps;

import static com.hedera.solo.operator.library.time.Timestamp.timestamp;
import static com.hedera.solo.operator.reconciler.common.ConditionStatus.True;

import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.reconciler.networkoperation.utils.NetworkOperationStepUtil;
import com.hedera.solo.operator.reconciler.networkoperation.utils.StepConditionType;
import io.fabric8.kubernetes.api.model.Condition;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import java.lang.invoke.MethodHandles;
import java.util.List;
import java.util.Objects;
import org.jboss.logging.Logger;

/**
 * This class implements the step for aborting a network upgrade operation. It handles the process of aborting the
 * upgrade by logging and updating the status accordingly.
 */
public class FreezeAbortStep implements ProcessorStep<NetworkOperation, NetworkOperationStatus> {
    public static final StepConditionType CONDITION_TYPE = new StepConditionType("FreezeAbort", "AbortNetworkUpgrade");
    public static final String SUCCESS_STATUS_MESSAGE = "Successfully aborted '%s/%s'";
    public static final String FAIL_STATUS_MESSAGE = "Failed to abort '%s/%s':'%s'";
    private static final Logger log = Logger.getLogger(MethodHandles.lookup().lookupClass());

    @Override
    public boolean isSuccessful(NetworkOperation resource, NetworkOperationStatus status) {
        return NetworkOperationStepUtil.isSuccessful(status, CONDITION_TYPE);
    }

    @Override
    public boolean shouldSkip(NetworkOperation resource, NetworkOperationStatus status) {
        return status.getPhase().equals(NetworkOperationStatus.Phase.Failed)
                || NetworkOperationStepUtil.isSuccessful(status, CONDITION_TYPE);
    }

    @Override
    public NetworkOperationStatus run(
            final NetworkOperation resource,
            final NetworkOperationStatus status,
            final Context<NetworkOperation> context) {

        try {
            Objects.requireNonNull(resource, "NetworkOperation is required");
            Objects.requireNonNull(resource.getSpec(), "NetworkOperationSpec is required");
            Objects.requireNonNull(resource.getSpec().getNetworkUpgrade(), "NetworkUpgrade is required");
            Objects.requireNonNull(status, "NetworkOperationStatus is required");
            Objects.requireNonNull(context, "Context<NetworkOperation> is required");

            log.infof(
                    "Aborting network operation: %s/%s [ %s ]",
                    resource.getMetadata().getNamespace(),
                    resource.getMetadata().getName(),
                    resource.getSpec().getNetworkUpgrade().getWorkflowStep());

            final Condition condition = new Condition(
                    timestamp(),
                    SUCCESS_STATUS_MESSAGE.formatted(
                            resource.getMetadata().getNamespace(),
                            resource.getMetadata().getName()),
                    resource.getMetadata().getGeneration(),
                    CONDITION_TYPE.reason(),
                    True,
                    CONDITION_TYPE.type());

            return NetworkOperationStepUtil.prepareStatus(
                    status, List.of(condition), NetworkOperationStatus.Phase.Succeeded);
        } catch (Throwable t) {
            log.errorf(
                    t,
                    FAIL_STATUS_MESSAGE,
                    resource.getMetadata().getNamespace(),
                    resource.getMetadata().getName(),
                    t.getMessage());
            return NetworkOperationStepUtil.prepareFailed(resource, status, t.getMessage(), CONDITION_TYPE);
        }
    }
}
