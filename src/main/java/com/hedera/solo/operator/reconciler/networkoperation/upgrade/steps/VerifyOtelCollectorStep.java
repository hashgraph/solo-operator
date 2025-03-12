// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.operator.library.time.Timestamp.timestamp;
import static com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.common.VerifyLayerHashes.selectedImage;
import static com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.common.VerifyLayerHashes.updateStatus;

import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.model.v1alpha1.networkoperationspec.networkupgrade.changesets.OtelCollector;
import com.hedera.solo.operator.model.v1alpha1.networkoperationstatus.SoftwareSourceStatus;
import com.hedera.solo.operator.reconciler.common.ConditionStatus;
import com.hedera.solo.operator.reconciler.networkoperation.utils.NetworkOperationStepUtil;
import io.fabric8.kubernetes.api.model.Condition;
import io.fabric8.kubernetes.api.model.ConditionBuilder;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import java.lang.invoke.MethodHandles;
import java.util.Objects;
import org.jboss.logging.Logger;

public class VerifyOtelCollectorStep implements ProcessorStep<NetworkOperation, NetworkOperationStatus> {
    public static final String CONDITION_TYPE = "ImageVerified: OtelCollector";
    static final String SUCCESS_STATUS_MESSAGE = "Image verification succeeded for 'otel-collector': %s";
    static final String FAIL_STATUS_MESSAGE = "Image verification failed for 'otel-collector'";
    private static final Logger log = Logger.getLogger(MethodHandles.lookup().lookupClass());

    @Override
    public NetworkOperationStatus run(
            NetworkOperation resource, NetworkOperationStatus status, Context<NetworkOperation> context) {
        Objects.requireNonNull(resource, "NetworkOperation resource is required");
        Objects.requireNonNull(status, "NetworkOperationStatus status is required");
        Objects.requireNonNull(status.getPhase(), "NetworkOperationStatus phase is required");
        Objects.requireNonNull(context, "Context<NetworkOperation> context is required");

        OtelCollector otelCollector =
                resource.getSpec().getNetworkUpgrade().getChangesets().getOtelCollector();

        if (Objects.isNull(otelCollector) || Objects.isNull(otelCollector.getSoftwareVersionSource())) {
            log.debugf(
                    "Ignore checking 'otel-collector' image as it is not specified in network operation: %s/%s",
                    resource.getMetadata().getNamespace(),
                    resource.getMetadata().getName());
            return status;
        }

        if (status.getPhase().equals(NetworkOperationStatus.Phase.Failed) || isDone(status)) {
            log.debugf(
                    "Skipping verification of 'otel-collector' image for network operation: %s/%s [ Phase: %s, Done: %s ]",
                    resource.getMetadata().getNamespace(),
                    resource.getMetadata().getName(),
                    status.getPhase(),
                    isDone(status));
            return status;
        }

        log.infof(
                "Verifying otel-collector image for network operation: %s/%s",
                resource.getMetadata().getNamespace(), resource.getMetadata().getName());

        String timestamp = timestamp();
        try {
            final NetworkOperationStatus updated = updateStatus(
                    resource,
                    status,
                    context.getClient(),
                    otelCollector.getSoftwareVersionSource(),
                    this::createCondition,
                    SoftwareSourceStatus::getOtelCollector,
                    SoftwareSourceStatus::setOtelCollector);

            return NetworkOperationStepUtil.checkLastCondition(updated);

        } catch (final Exception ex) {
            log.errorf(
                    ex,
                    "Failed to verify otel-collector image for network operation: %s/%s",
                    resource.getMetadata().getNamespace(),
                    resource.getMetadata().getName());

            return edit(status)
                    .with(s -> s.addToConditions(new ConditionBuilder()
                                    .withObservedGeneration(
                                            resource.getMetadata().getGeneration())
                                    .withType(CONDITION_TYPE)
                                    .withStatus(ConditionStatus.False)
                                    .withLastTransitionTime(timestamp)
                                    .withMessage("%s: %s".formatted(FAIL_STATUS_MESSAGE, ex.getMessage()))
                                    .build())
                            .withPhase(NetworkOperationStatus.Phase.Failed))
                    .build();
        }
    }

    Condition createCondition(final NetworkOperationStatus status) {
        String image = selectedImage(status.getSoftwareSourceStatus().getOtelCollector());
        String timestamp = timestamp();
        return image == null
                ? new Condition(timestamp, FAIL_STATUS_MESSAGE, null, null, ConditionStatus.False, CONDITION_TYPE)
                : new Condition(
                        null,
                        SUCCESS_STATUS_MESSAGE.formatted(image),
                        null,
                        null,
                        ConditionStatus.True,
                        CONDITION_TYPE);
    }

    public static boolean isDone(NetworkOperationStatus status) {
        return NetworkOperationStepUtil.isDone(status, CONDITION_TYPE);
    }
}
