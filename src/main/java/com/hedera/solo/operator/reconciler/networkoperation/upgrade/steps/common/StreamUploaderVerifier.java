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
import io.fabric8.kubernetes.api.model.Condition;
import java.util.List;
import java.util.function.BiConsumer;
import java.util.function.Function;

public final class StreamUploaderVerifier {
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

    public static Condition prepareCondition(
            final NetworkOperationStatus status,
            final String containerName,
            final String successMessageTemplate,
            final String failedMessageTemplate,
            final String conditionType) {
        final String image = selectedImage(getter(containerName).apply(status.getSoftwareSourceStatus()));
        final String timestamp = timestamp();
        return edit(new Condition())
                .with(builder -> builder.withType(conditionType)
                        .withLastTransitionTime(timestamp)
                        .withMessage(
                                image == null
                                        ? failedMessageTemplate.formatted(containerName)
                                        : successMessageTemplate.formatted(containerName, image))
                        .withStatus(image == null ? ConditionStatus.False : ConditionStatus.True))
                .build();
    }
}
