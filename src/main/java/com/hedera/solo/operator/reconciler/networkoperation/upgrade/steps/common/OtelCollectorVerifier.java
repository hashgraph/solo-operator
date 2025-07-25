// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.common;

import static com.hedera.solo.operator.library.time.Timestamp.timestamp;
import static com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.common.VerifyLayerHashes.selectedImage;

import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.reconciler.common.ConditionStatus;
import com.hedera.solo.operator.reconciler.networkoperation.utils.StepConditionType;
import io.fabric8.kubernetes.api.model.Condition;

/** The OtelCollectorVerifier class provides a method to create a condition based on the status of an OTel collector. */
public final class OtelCollectorVerifier {
    /** Private constructor to prevent instantiation of this utility class. */
    private OtelCollectorVerifier() {}

    /**
     * Creates a condition for the OTel collector based on its status.
     *
     * @param status The current status of the network operation, which includes the software source status of the OTel
     *     collector.
     * @param successMessageTemplate A template string that will be used to format the success message if the image is
     *     not null.
     * @param failMessage The failure message to be used if the image is null.
     * @param conditionType An instance of StepConditionType which contains the type and reason for the condition.
     * @return A Condition object representing the state of the OTel collector
     */
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
