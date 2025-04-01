// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.check.steps;

import com.hedera.solo.operator.library.k8s.CustomResourceManager;
import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.reconciler.common.ConditionStatus;
import com.hedera.solo.operator.reconciler.networkoperation.utils.NetworkOperationStepUtil;
import com.hedera.solo.operator.reconciler.networkoperation.utils.StepConditionType;
import io.fabric8.kubernetes.api.model.ConditionBuilder;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import org.jboss.logging.Logger;

import java.lang.invoke.MethodHandles;
import java.util.List;
import java.util.Objects;

import static com.hedera.solo.operator.library.time.Timestamp.timestamp;

/**
 * CheckConsensusPodConfigurationStep checks that consensus capsule configuration are rolled out during network upgrade.
 *
 * <p>It needs to inspect the CR and performs the necessary checks to ensure that the consensus nodes are running with
 * correct version and configuration files.
 */
public class CheckConsensusCapsuleConfigurationStep implements ProcessorStep<NetworkOperation, NetworkOperationStatus> {
    public static final StepConditionType CONDITION_TYPE =
            new StepConditionType("ConfigurationVerified", "ConsensusCapsule");
    public static final String SUCCESS_STATUS_MESSAGE = "Successfully verified consensus capsule configuration";
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

            // check consensus changeset configuration

            // check record-stream-uploader changeset configuration

            // check event-stream-uploader changeset configuration

            // check backup-uploader changeset configuration

            // check record-stream-sidecar changeset configuration

            // check otel-collector changeset configuration

            // check configuration changes

            return NetworkOperationStepUtil.prepareStatus(
                    status,
                    List.of(new ConditionBuilder()
                            .withType(CONDITION_TYPE.type())
                            .withReason(CONDITION_TYPE.reason())
                            .withLastTransitionTime(timestamp())
                            .withObservedGeneration(resource.getMetadata().getGeneration())
                            .withMessage(SUCCESS_STATUS_MESSAGE)
                            .withStatus(ConditionStatus.True)
                            .build()));
        } catch (Throwable t) {
            log.errorf(t, FAIL_STATUS_MESSAGE, t.getMessage());
            return NetworkOperationStepUtil.prepareFailed(resource, status, t.getMessage(), CONDITION_TYPE);
        }
    }

    public static boolean isDone(NetworkOperationStatus status) {
        return NetworkOperationStepUtil.isSuccessful(status, CONDITION_TYPE);
    }
}
