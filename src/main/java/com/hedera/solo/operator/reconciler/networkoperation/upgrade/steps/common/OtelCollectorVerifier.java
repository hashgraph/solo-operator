// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.common;

import static com.hedera.solo.operator.library.time.Timestamp.timestamp;
import static com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.common.VerifyLayerHashes.selectedImage;

import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.reconciler.common.ConditionStatus;
import com.hedera.solo.operator.reconciler.networkoperation.utils.StepConditionType;
import io.fabric8.kubernetes.api.model.Condition;

public final class OtelCollectorVerifier {

    public static Condition createCondition(
            final NetworkOperationStatus status,
            final String successMessageTemplate,
            final String failMessage,
            final StepConditionType conditionType) {
        final String image = selectedImage(status.getSoftwareSourceStatus().getOtelCollector());
        final String timestamp = timestamp();
        return image == null
                ? new Condition(
                        timestamp,
                        failMessage,
                        null,
                        conditionType.reason(),
                        ConditionStatus.False,
                        conditionType.type())
                : new Condition(
                        timestamp,
                        successMessageTemplate.formatted(image),
                        null,
                        conditionType.reason(),
                        ConditionStatus.True,
                        conditionType.type());
    }
}
