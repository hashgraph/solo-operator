// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps;

import static com.hedera.solo.operator.library.time.Timestamp.timestamp;
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

public class VerifyOtelCollector implements ProcessorStep<NetworkOperation, NetworkOperationStatus> {
    @Override
    public NetworkOperationStatus run(
            NetworkOperation resource, NetworkOperationStatus status, Context<NetworkOperation> context) {
        OtelCollector otelCollector =
                resource.getSpec().getNetworkUpgrade().getChangesets().getOtelCollector();
        if (otelCollector == null) {
            return status;
        }
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
        String type = getClass().getSimpleName();
        String image = selectedImage(status.getSoftwareSourceStatus().getConsensusNode());
        String timestamp = timestamp();
        return image == null
                ? new Condition(
                        timestamp,
                        "image verification failed for 'otel-collector'",
                        null,
                        null,
                        ConditionStatus.False,
                        type)
                : new Condition(
                        timestamp,
                        "image verification succeeded for 'otel-collector': " + image,
                        null,
                        null,
                        ConditionStatus.True,
                        type);
    }
}
