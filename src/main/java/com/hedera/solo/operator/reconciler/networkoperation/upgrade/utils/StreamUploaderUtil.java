// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.utils;

import com.hedera.solo.operator.model.commonspec.StreamUploaderChangeset;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.reconciler.common.Constants;

public class StreamUploaderUtil {
    public static StreamUploaderChangeset findChangeset(NetworkOperation resource, String name) {
        return switch (name) {
            case Constants.TPL_VAR_RECORD_STREAM_UPLOADER -> resource.getSpec()
                    .getNetworkUpgrade()
                    .getChangesets()
                    .getRecordStreamUploaderChangeset();
            case Constants.TPL_VAR_RECORD_STREAM_SIDECAR_FILES_UPLOADER -> resource.getSpec()
                    .getNetworkUpgrade()
                    .getChangesets()
                    .getRecordStreamSidecarUploaderChangeset();
            case Constants.TPL_VAR_EVENT_STREAM_UPLOADER -> resource.getSpec()
                    .getNetworkUpgrade()
                    .getChangesets()
                    .getEventsStreamUploaderChangeset();
            case Constants.TPL_VAR_BACKUP_UPLOADER -> resource.getSpec()
                    .getNetworkUpgrade()
                    .getChangesets()
                    .getBackupUploaderChangeset();
            default -> null;
        };
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

    public static String computeConditionType(final String containerName) {
        return "ImageVerified: " + toStreamUploaderType(containerName);
    }
}
