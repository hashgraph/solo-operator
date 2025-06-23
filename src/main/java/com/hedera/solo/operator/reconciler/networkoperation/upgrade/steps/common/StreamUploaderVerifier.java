// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.common;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.operator.library.time.Timestamp.timestamp;
import static com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.common.VerifyLayerHashes.selectedImage;

import com.hedera.solo.operator.model.commonspec.ImageRepositoryStatus;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.model.v1alpha1.networkoperationstatus.SoftwareSourceStatus;
import com.hedera.solo.operator.reconciler.common.ConditionStatus;
import com.hedera.solo.operator.reconciler.common.Constants;
import com.hedera.solo.operator.reconciler.networkoperation.utils.StepConditionType;
import io.fabric8.kubernetes.api.model.Condition;
import java.util.List;
import java.util.function.BiConsumer;
import java.util.function.Function;

/** This class provides utility methods for handling stream uploaders in the network operation upgrade steps. */
public final class StreamUploaderVerifier {

    /**
     * Returns a getter function based on the container name.
     *
     * @param containerName The name of the container whose status is to be retrieved.
     * @return A Function that retrieves the SoftwareSourceStatus for the specified container
     * @throws IllegalStateException if the container name is unrecognized.
     */
    public static Function<SoftwareSourceStatus, List<ImageRepositoryStatus>> getter(String containerName) {
        return switch (containerName) {
            case Constants.TPL_VAR_BACKUP_UPLOADER -> SoftwareSourceStatus::getBackupUploader;
            case Constants.TPL_VAR_EVENT_STREAM_UPLOADER -> SoftwareSourceStatus::getEventsStreamUploader;
            case Constants.TPL_VAR_RECORD_STREAM_SIDECAR_FILES_UPLOADER -> SoftwareSourceStatus
                    ::getRecordStreamSidecarUploader;
            case Constants.TPL_VAR_RECORD_STREAM_UPLOADER -> SoftwareSourceStatus::getRecordStreamUploader;
            default -> throw new IllegalStateException("Unrecognized stream uploader container name: " + containerName);
        };
    }

    /**
     * Returns a setter function based on the container name.
     *
     * @param containerName The name of the container whose status is to be set.
     * @return A BiConsumer that sets the SoftwareSourceStatus for the specified container
     * @throws IllegalStateException if the container name is unrecognized.
     */
    public static BiConsumer<SoftwareSourceStatus, List<ImageRepositoryStatus>> setter(String containerName) {
        return switch (containerName) {
            case Constants.TPL_VAR_BACKUP_UPLOADER -> SoftwareSourceStatus::setBackupUploader;
            case Constants.TPL_VAR_EVENT_STREAM_UPLOADER -> SoftwareSourceStatus::setEventsStreamUploader;
            case Constants.TPL_VAR_RECORD_STREAM_SIDECAR_FILES_UPLOADER -> SoftwareSourceStatus
                    ::setRecordStreamSidecarUploader;
            case Constants.TPL_VAR_RECORD_STREAM_UPLOADER -> SoftwareSourceStatus::setRecordStreamUploader;
            default -> throw new IllegalStateException("Unrecognized stream uploader container name: " + containerName);
        };
    }

    /**
     * Prepares a condition object based on the status and container name provided.
     *
     * @param status The NetworkOperationStatus containing the software source status.
     * @param containerName The name of the container whose verification is being performed.
     * @param successMessageTemplate Template for the success message.
     * @param failedMessageTemplate Template for the failure message.
     * @param conditionType The StepConditionType representing the type and reason of the condition.
     * @return A Condition object with appropriate status, message, and timestamp based on the verification result
     */
    public static Condition prepareCondition(
            final NetworkOperationStatus status,
            final String containerName,
            final String successMessageTemplate,
            final String failedMessageTemplate,
            final StepConditionType conditionType) {
        final String image = selectedImage(getter(containerName).apply(status.getSoftwareSourceStatus()));
        final String timestamp = timestamp();
        return edit(new Condition())
                .with(builder -> builder.withType(conditionType.type())
                        .withReason(conditionType.reason())
                        .withLastTransitionTime(timestamp)
                        .withMessage(
                                image == null
                                        ? failedMessageTemplate.formatted(containerName)
                                        : successMessageTemplate.formatted(containerName, image))
                        .withStatus(image == null ? ConditionStatus.False : ConditionStatus.True))
                .build();
    }
}
