// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.check.steps;

import com.hedera.solo.operator.library.k8s.CustomResourceManager;
import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.reconciler.networkoperation.utils.NetworkOperationStepUtil;
import com.hedera.solo.operator.reconciler.networkoperation.utils.StepConditionType;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import java.lang.invoke.MethodHandles;
import java.util.List;
import java.util.Objects;
import org.jboss.logging.Logger;

/**
 * CheckConsensusPodConfigurationStep checks that consensus pod configuration are rolled out
 *
 * <p>It needs to inspect the CR and performs the necessary checks to ensure that the consensus nodes are running with
 * correct version and configuration files.
 */
public class CheckConsensusCapsuleConfigurationStep implements ProcessorStep<NetworkOperation, NetworkOperationStatus> {
    public static final StepConditionType CONDITION_TYPE =
            new StepConditionType("ConfigurationVerified", "ConsensusCapsule");
    public static final String SUCCESS_STATUS_MESSAGE = "Successfully verified consensus capsule configuration: %s";
    public static final String FAIL_STATUS_MESSAGE = "Failed to verify consensus capsule configuration: %s";
    private static final Logger log = Logger.getLogger(MethodHandles.lookup().lookupClass());

    private final CustomResourceManager customResourceManager;

    public CheckConsensusCapsuleConfigurationStep(CustomResourceManager customResourceManager) {
        this.customResourceManager = customResourceManager;
    }

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
            NetworkOperation resource, NetworkOperationStatus status, Context<NetworkOperation> context) {
        try {
            Objects.requireNonNull(resource, "NetworkOperation resource is required");
            Objects.requireNonNull(status, "NetworkOperationStatus status is required");
            Objects.requireNonNull(status.getPhase(), "NetworkOperationStatus phase is required");
            Objects.requireNonNull(context, "Context<NetworkOperation> context is required");

            return NetworkOperationStepUtil.prepareStatus(status, List.of()); // FIXME implement the logic
        } catch (Throwable t) {
            log.errorf(t, FAIL_STATUS_MESSAGE, t.getMessage());
            return NetworkOperationStepUtil.prepareFailed(resource, status, t.getMessage(), CONDITION_TYPE);
        }
    }

    public static boolean isDone(NetworkOperationStatus status) {
        return NetworkOperationStepUtil.isSuccessful(status, CONDITION_TYPE);
    }
}
