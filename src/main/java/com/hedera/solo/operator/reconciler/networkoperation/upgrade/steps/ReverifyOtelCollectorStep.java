// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps;

import static com.hedera.solo.operator.library.time.Timestamp.timestamp;
import static com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.common.VerifyLayerHashes.selectedImage;
import static com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.common.VerifyLayerHashes.updateStatus;

import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.model.v1alpha1.networkoperationspec.networkupgrade.changesets.OtelCollector;
import com.hedera.solo.operator.model.v1alpha1.networkoperationstatus.SoftwareSourceStatus;
import com.hedera.solo.operator.reconciler.common.ConditionStatus;
import com.hedera.solo.operator.reconciler.networkoperation.utils.ConsensusPodSidecarUtil;
import com.hedera.solo.operator.reconciler.networkoperation.utils.NetworkOperationStepUtil;
import io.fabric8.kubernetes.api.model.Condition;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import java.lang.invoke.MethodHandles;
import java.util.Objects;
import org.jboss.logging.Logger;

public class ReverifyOtelCollectorStep implements ProcessorStep<NetworkOperation, NetworkOperationStatus> {
    public static final String CONDITION_TYPE = "ImageReVerified: OtelCollector";
    public static final String SUCCESS_STATUS_MESSAGE = "Image re-verification succeeded for 'otel-collector': %s";
    public static final String FAIL_STATUS_MESSAGE = "Image re-verification failed for 'otel-collector'";
    private static final Logger log = Logger.getLogger(MethodHandles.lookup().lookupClass());

    @Override
    public boolean isSuccessful(NetworkOperation resource, NetworkOperationStatus status) {
        return NetworkOperationStepUtil.isSuccessful(status, CONDITION_TYPE);
    }

    @Override
    public boolean shouldSkip(NetworkOperation resource, NetworkOperationStatus status) {
        if (!ConsensusPodSidecarUtil.hasOtelChangeset(resource)) {
            return true;
        }

        return status.getPhase().equals(NetworkOperationStatus.Phase.Failed)
                || NetworkOperationStepUtil.isSuccessful(status, CONDITION_TYPE);
    }

    @Override
    public NetworkOperationStatus run(
            NetworkOperation resource, NetworkOperationStatus status, Context<NetworkOperation> context) {
        try {
            OtelCollector otelCollector =
                    resource.getSpec().getNetworkUpgrade().getChangesets().getOtelCollector();

            if (Objects.isNull(otelCollector) || Objects.isNull(otelCollector.getSoftwareVersionSource())) {
                log.debugf(
                        "Ignore checking 'otel-collector' image as it is not specified in network operation: %s/%s [ %s ]",
                        resource.getMetadata().getNamespace(),
                        resource.getMetadata().getName(),
                        resource.getSpec().getNetworkUpgrade().getWorkflowStep());
                return status;
            }

            log.infof(
                    "Re-verifying 'otel-collector' image for network operation: %s/%s [ %s ]",
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

            log.infof(
                    "Finished re-verification of 'otel-collector' image for network operation: %s/%s [ %s ]",
                    resource.getMetadata().getNamespace(),
                    resource.getMetadata().getName(),
                    resource.getSpec().getNetworkUpgrade().getWorkflowStep());

            return NetworkOperationStepUtil.checkLastCondition(updated);
        } catch (Throwable t) {
            log.errorf(t, FAIL_STATUS_MESSAGE, t.getMessage());
            return NetworkOperationStepUtil.prepareFailed(resource, status, t.getMessage(), CONDITION_TYPE);
        }
    }

    private Condition createCondition(final NetworkOperationStatus status) {
        String image = selectedImage(status.getSoftwareSourceStatus().getOtelCollector());
        String timestamp = timestamp();
        return image == null
                ? new Condition(timestamp, FAIL_STATUS_MESSAGE, null, null, ConditionStatus.False, CONDITION_TYPE)
                : new Condition(
                        timestamp,
                        SUCCESS_STATUS_MESSAGE.formatted(image),
                        null,
                        null,
                        ConditionStatus.True,
                        CONDITION_TYPE);
    }

    public static boolean isDone(NetworkOperationStatus status) {
        return NetworkOperationStepUtil.isSuccessful(status, CONDITION_TYPE);
    }
}
