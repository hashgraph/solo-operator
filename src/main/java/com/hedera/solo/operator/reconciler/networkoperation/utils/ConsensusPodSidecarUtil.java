// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.utils;

import com.hedera.solo.operator.model.commonspec.StreamUploaderChangeset;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.networkoperationspec.networkupgrade.Changesets;
import com.hedera.solo.operator.reconciler.common.Constants;
import java.util.Objects;

public final class ConsensusPodSidecarUtil {
    public static StreamUploaderChangeset findStreamUploaderChangeset(NetworkOperation resource, String name) {
        final Changesets changesets = resource.getSpec().getNetworkUpgrade().getChangesets();
        if (changesets == null) {
            return null;
        }

        return switch (name) {
            case Constants.TPL_VAR_RECORD_STREAM_UPLOADER -> changesets.getRecordStreamUploaderChangeset();
            case Constants.TPL_VAR_RECORD_STREAM_SIDECAR_FILES_UPLOADER -> changesets
                    .getRecordStreamSidecarUploaderChangeset();
            case Constants.TPL_VAR_EVENT_STREAM_UPLOADER -> changesets.getEventsStreamUploaderChangeset();
            case Constants.TPL_VAR_BACKUP_UPLOADER -> changesets.getBackupUploaderChangeset();
            default -> null;
        };
    }

    public static boolean hasStreamUploaderChangeset(NetworkOperation resource, String name) {
        final StreamUploaderChangeset changeset = findStreamUploaderChangeset(resource, name);
        return Objects.nonNull(changeset) && Objects.nonNull(changeset.getSoftwareVersionSource());
    }

    public static String toStreamUploaderType(String containerName) {
        return switch (containerName) {
            case Constants.TPL_VAR_BACKUP_UPLOADER -> "BackupUploader";
            case Constants.TPL_VAR_EVENT_STREAM_UPLOADER -> "EventsStreamUploader";
            case Constants.TPL_VAR_RECORD_STREAM_SIDECAR_FILES_UPLOADER -> "RecordStreamSidecarUploader";
            case Constants.TPL_VAR_RECORD_STREAM_UPLOADER -> "RecordStreamUploader";
            default -> throw new IllegalStateException("Unrecognized stream uploader container name: " + containerName);
        };
    }

    public static boolean hasOtelChangeset(NetworkOperation resource) {
        if (Objects.isNull(resource)
                || Objects.isNull(resource.getSpec())
                || Objects.isNull(resource.getSpec().getNetworkUpgrade())
                || Objects.isNull(resource.getSpec().getNetworkUpgrade().getChangesets())) {
            return false;
        }

        return Objects.nonNull(
                resource.getSpec().getNetworkUpgrade().getChangesets().getOtelCollector());
    }
}
