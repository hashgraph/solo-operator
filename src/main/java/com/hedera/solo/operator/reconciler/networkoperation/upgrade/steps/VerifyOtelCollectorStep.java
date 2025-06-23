// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps;

import static com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.common.VerifyLayerHashes.updateStatus;
import static com.hedera.solo.operator.reconciler.networkoperation.utils.ConsensusPodSidecarUtil.hasOtelChangeset;

import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.model.v1alpha1.networkoperationspec.networkupgrade.changesets.OtelCollector;
import com.hedera.solo.operator.model.v1alpha1.networkoperationstatus.SoftwareSourceStatus;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.common.OtelCollectorVerifier;
import com.hedera.solo.operator.reconciler.networkoperation.utils.NetworkOperationStepUtil;
import com.hedera.solo.operator.reconciler.networkoperation.utils.StepConditionType;
import io.fabric8.kubernetes.api.model.Condition;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import java.lang.invoke.MethodHandles;
import java.util.List;
import java.util.Objects;
import org.jboss.logging.Logger;

/** This class handles the verification of the OtelCollector image for a network operation. */
public class VerifyOtelCollectorStep implements ProcessorStep<NetworkOperation, NetworkOperationStatus> {
    public static final StepConditionType CONDITION_TYPE = new StepConditionType("ImageVerified", "OtelCollector");
    public static final String SUCCESS_STATUS_MESSAGE = "Image verification succeeded for 'otel-collector': %s";
    public static final String FAIL_STATUS_MESSAGE = "Image verification failed for 'otel-collector'";
    private static final Logger log = Logger.getLogger(MethodHandles.lookup().lookupClass());

    @Override
    public boolean isSuccessful(NetworkOperation resource, NetworkOperationStatus status) {
        return NetworkOperationStepUtil.isSuccessful(status, CONDITION_TYPE);
    }

    @Override
    public boolean shouldSkip(NetworkOperation resource, NetworkOperationStatus status) {
        if (!hasOtelChangeset(resource)) {
            return true;
        }

        return status.getPhase().equals(NetworkOperationStatus.Phase.Failed)
                || NetworkOperationStepUtil.isSuccessful(status, CONDITION_TYPE);
    }

    @Override
    public NetworkOperationStatus run(
            NetworkOperation resource, NetworkOperationStatus status, Context<NetworkOperation> context) {
        try {
            Objects.requireNonNull(resource, "NetworkOperation is required");
            Objects.requireNonNull(resource.getSpec(), "NetworkOperationSpec is required");
            Objects.requireNonNull(resource.getSpec().getNetworkUpgrade(), "NetworkUpgrade is required");
            Objects.requireNonNull(status, "NetworkOperationStatus is required");
            Objects.requireNonNull(status.getPhase(), "NetworkOperationStatus is required");
            Objects.requireNonNull(context, "Context<NetworkOperation> is required");

            OtelCollector otelCollector =
                    resource.getSpec().getNetworkUpgrade().getChangesets().getOtelCollector();

            if (Objects.isNull(otelCollector) || Objects.isNull(otelCollector.getSoftwareVersionSource())) {
                log.debugf(
                        "Ignore checking 'otel-collector' image as it is not specified in network operation: %s/%s",
                        resource.getMetadata().getNamespace(),
                        resource.getMetadata().getName(),
                        resource.getSpec().getNetworkUpgrade().getWorkflowStep());
                return status;
            }

            log.infof(
                    "Verifying otel-collector image for network operation: %s/%s [ %s ]",
                    resource.getMetadata().getNamespace(),
                    resource.getMetadata().getName(),
                    resource.getSpec().getNetworkUpgrade().getWorkflowStep());

            final NetworkOperationStatus updated = updateStatus(
                    resource,
                    status,
                    context.getClient(),
                    otelCollector.getSoftwareVersionSource(),
                    this::createCondition,
                    SoftwareSourceStatus::getOtelCollector,
                    SoftwareSourceStatus::setOtelCollector);

            if (!isDone(updated)) {
                log.warnf(
                        "Failed to verify otel-collector image for network operation: %s/%s [ %s ]",
                        resource.getMetadata().getNamespace(),
                        resource.getMetadata().getName(),
                        resource.getSpec().getNetworkUpgrade().getWorkflowStep());

                return NetworkOperationStepUtil.prepareStatus(updated, List.of(), NetworkOperationStatus.Phase.Failed);
            }

            log.infof(
                    "Successfully verified 'otel-collector' image for network operation: %s/%s [ %s ]",
                    resource.getMetadata().getNamespace(),
                    resource.getMetadata().getName(),
                    resource.getSpec().getNetworkUpgrade().getWorkflowStep());

            return NetworkOperationStepUtil.prepareStatus(updated, List.of());
        } catch (Throwable t) {
            log.errorf(t, FAIL_STATUS_MESSAGE, t.getMessage());
            return NetworkOperationStepUtil.prepareFailed(resource, status, t.getMessage(), CONDITION_TYPE);
        }
    }

    /**
     * Creates a condition for the verification of the OtelCollector image.
     *
     * @param status The current status of the NetworkOperation.
     * @return A Condition object representing the condition of the verification process
     */
    Condition createCondition(final NetworkOperationStatus status) {
        return OtelCollectorVerifier.createCondition(
                status, SUCCESS_STATUS_MESSAGE, FAIL_STATUS_MESSAGE, CONDITION_TYPE);
    }

    /**
     * Checks if the verification of the OtelCollector image is done by checking the status condition type.
     *
     * @param status The current status of the NetworkOperation.
     * @return true if the verification is done, false otherwise
     */
    public static boolean isDone(NetworkOperationStatus status) {
        return NetworkOperationStepUtil.isSuccessful(status, CONDITION_TYPE);
    }
}
