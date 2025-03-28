// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps;

import static com.hedera.solo.operator.library.generic.Nullable.orEmptyList;
import static com.hedera.solo.operator.library.time.Timestamp.timestamp;
import static com.hedera.solo.operator.reconciler.common.ConditionStatus.True;

import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.reconciler.networkoperation.utils.NetworkOperationStepUtil;
import io.fabric8.kubernetes.api.model.Condition;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import java.lang.invoke.MethodHandles;
import java.util.ArrayList;
import java.util.List;
import java.util.Objects;
import java.util.stream.IntStream;
import org.jboss.logging.Logger;

public class FreezeAbortStep implements ProcessorStep<NetworkOperation, NetworkOperationStatus> {

    public static final String CONDITION_TYPE = "FreezeAbort";
    public static final String SUCCESS_STATUS_MESSAGE = "Successfully aborted '%s'";
    public static final String FAIL_STATUS_MESSAGE = "Failed to abort '%s':'%s'";
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
            Objects.requireNonNull(resource, "NetworkOperation resource is required");
            Objects.requireNonNull(status, "NetworkOperationStatus status is required");
            Objects.requireNonNull(context, "Context<NetworkOperation> context is required");

            log.infof(
                    "Aborting network operation: %s/%s [ %s ]",
                    resource.getMetadata().getNamespace(),
                    resource.getMetadata().getName(),
                    resource.getSpec().getNetworkUpgrade().getWorkflowStep());

            List<Condition> conditions = new ArrayList<>(orEmptyList(() -> status.getConditions()));
            int index = IntStream.range(0, conditions.size())
                    .filter(i -> CONDITION_TYPE.equals(conditions.get(i).getType()))
                    .findFirst()
                    .orElse(-1);
            if (index == -1) {
                conditions.add(new Condition(
                        timestamp(), null, resource.getMetadata().getGeneration(), null, True, CONDITION_TYPE));
            } else {
                conditions.set(
                        index,
                        conditions
                                .get(index)
                                .edit()
                                .withObservedGeneration(resource.getMetadata().getGeneration())
                                .build());
            }

            log.infof(
                    "Successfully aborted network operation: %s/%s [ %s ]",
                    resource.getMetadata().getNamespace(),
                    resource.getMetadata().getName(),
                    resource.getSpec().getNetworkUpgrade().getWorkflowStep());

            return status.edit()
                    .withPhase(NetworkOperationStatus.Phase.Succeeded)
                    .withConditions(conditions)
                    .build();
        } catch (Throwable t) {
            log.errorf(
                    t,
                    FAIL_STATUS_MESSAGE.formatted(
                            resource.getMetadata().getNamespace(),
                            resource.getMetadata().getName()),
                    t.getMessage());
            return NetworkOperationStepUtil.prepareFailed(resource, status, t.getMessage(), CONDITION_TYPE);
        }
    }
}
