// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps;

import static com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.VerifyLayerHashes.selectedImage;
import static com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.VerifyLayerHashes.updateStatus;

import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.model.v1alpha1.networkoperationspec.networkupgrade.changesets.OtelCollector;
import com.hedera.solo.operator.model.v1alpha1.networkoperationstatus.SoftwareSourceStatus;
import com.hedera.solo.operator.reconciler.common.ConditionStatus;
import io.fabric8.kubernetes.api.model.Condition;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import java.lang.invoke.MethodHandles;
import java.util.Objects;
import org.jboss.logging.Logger;

public class VerifyOtelCollectorStep implements ProcessorStep<NetworkOperation, NetworkOperationStatus> {
    public static final String CONDITION_TYPE = "ImageVerified: OtelCollector";
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

        if (Objects.isNull(otelCollector)
                || Objects.isNull(otelCollector.getSoftwareVersionSource())
                || status.getPhase().equals(NetworkOperationStatus.Phase.Failed)
                || isDone(status)) {
            log.infof(
                    "Skipping otel-collector image for network operation: %s/%s",
                    resource.getMetadata().getNamespace(),
                    resource.getMetadata().getName());
            return status;
        }

        log.infof(
                "Verifying otel-collector image for network operation: %s/%s",
                resource.getMetadata().getNamespace(), resource.getMetadata().getName());
        return updateStatus(
                resource,
                status,
                context.getClient(),
                otelCollector.getSoftwareVersionSource(),
                this::createCondition,
                SoftwareSourceStatus::getOtelCollector,
                SoftwareSourceStatus::setOtelCollector);
    }

    Condition createCondition(final NetworkOperationStatus status) {
        String image = selectedImage(status.getSoftwareSourceStatus().getConsensusNode());
        return image == null
                ? new Condition(null, "image verification failed", null, null, ConditionStatus.False, CONDITION_TYPE)
                : new Condition(
                        null,
                        "Successfully verified image: " + image,
                        null,
                        null,
                        ConditionStatus.True,
                        CONDITION_TYPE);
    }

    public static boolean isDone(NetworkOperationStatus status) {
        if (status == null || status.getConditions() == null) {
            return false;
        }

        return status.getConditions().stream()
                .anyMatch(
                        c -> c.getType().equals(CONDITION_TYPE) && c.getStatus().equals(ConditionStatus.True));
    }
}
