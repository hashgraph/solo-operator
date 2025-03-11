// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.operator.library.time.Timestamp.timestamp;
import static com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.VerifyLayerHashes.selectedImage;
import static com.hedera.solo.operator.reconciler.networkoperation.upgrade.utils.StreamUploaderUtil.computeConditionType;

import com.hedera.solo.operator.model.commonspec.ImageRepositoryStatus;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.model.v1alpha1.networkoperationstatus.SoftwareSourceStatus;
import com.hedera.solo.operator.reconciler.common.ConditionStatus;
import com.hedera.solo.operator.reconciler.common.Constants;
import io.fabric8.kubernetes.api.model.Condition;
import java.util.List;
import java.util.function.BiConsumer;
import java.util.function.Function;

public class StreamUploaderVerifier {
    static final String SUCCESS_STATUS_MESSAGE =
            "Stream uploader image verification succeeded for '%s'. Selected for deployment: %s";
    static final String FAIL_STATUS_MESSAGE = "Stream uploader image verification failed for '%s'";

    final String containerName;

    public StreamUploaderVerifier(final String name) {
        containerName = name;
    }

    Function<SoftwareSourceStatus, List<ImageRepositoryStatus>> getter() {
        return switch (containerName) {
            case Constants.TPL_VAR_BACKUP_UPLOADER -> SoftwareSourceStatus::getBackupUploader;
            case Constants.TPL_VAR_EVENT_STREAM_UPLOADER -> SoftwareSourceStatus::getEventsStreamUploader;
            case Constants.TPL_VAR_RECORD_STREAM_SIDECAR_FILES_UPLOADER -> SoftwareSourceStatus
                    ::getRecordStreamSidecarUploader;
            case Constants.TPL_VAR_RECORD_STREAM_UPLOADER -> SoftwareSourceStatus::getRecordStreamUploader;
            default -> throw new IllegalStateException("Unrecognized stream uploader container name: " + containerName);
        };
    }

    BiConsumer<SoftwareSourceStatus, List<ImageRepositoryStatus>> setter() {
        return switch (containerName) {
            case Constants.TPL_VAR_BACKUP_UPLOADER -> SoftwareSourceStatus::setBackupUploader;
            case Constants.TPL_VAR_EVENT_STREAM_UPLOADER -> SoftwareSourceStatus::setEventsStreamUploader;
            case Constants.TPL_VAR_RECORD_STREAM_SIDECAR_FILES_UPLOADER -> SoftwareSourceStatus
                    ::setRecordStreamSidecarUploader;
            case Constants.TPL_VAR_RECORD_STREAM_UPLOADER -> SoftwareSourceStatus::setRecordStreamUploader;
            default -> throw new IllegalStateException("Unrecognized stream uploader container name: " + containerName);
        };
    }

    Condition createCondition(final NetworkOperationStatus status) {
        String type = computeConditionType(containerName);
        String image = selectedImage(getter().apply(status.getSoftwareSourceStatus()));
        String timestamp = timestamp();
        return edit(new Condition())
                .with(builder -> builder.withType(type)
                        .withLastTransitionTime(timestamp)
                        .withMessage(
                                image == null
                                        ? FAIL_STATUS_MESSAGE.formatted(containerName)
                                        : SUCCESS_STATUS_MESSAGE.formatted(containerName, image))
                        .withStatus(image == null ? ConditionStatus.False : ConditionStatus.True))
                .build();
    }
}
