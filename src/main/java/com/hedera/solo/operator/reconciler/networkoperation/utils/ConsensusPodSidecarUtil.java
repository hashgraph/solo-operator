// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.utils;

import com.hedera.solo.operator.model.commonspec.StreamUploaderChangeset;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.networkoperationspec.networkupgrade.Changesets;
import com.hedera.solo.operator.reconciler.common.Constants;
import java.util.Objects;

/**
 * This utility class provides methods to find and manipulate changesets related to stream uploaders in a
 * NetworkOperation.
 */
public final class ConsensusPodSidecarUtil {
    /** Private constructor to prevent instantiation of this utility class. */
    private ConsensusPodSidecarUtil() {}

    /**
     * Finds the StreamUploaderChangeset based on the given name.
     *
     * @param resource The NetworkOperation containing the network upgrade specification.
     * @param name The name of the changeset to find, which can be one of: - Constants.TPL_VAR_RECORD_STREAM_UPLOADER -
     *     Constants.TPL_VAR_RECORD_STREAM_SIDECAR_FILES_UPLOADER - Constants.TPL_VAR_EVENT_STREAM_UPLOADER -
     *     Constants.TPL_VAR_BACKUP_UPLOADER
     * @return The StreamUploaderChangeset if found, otherwise null
     */
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

    /**
     * Checks if a StreamUploaderChangeset exists and is not null for the given name.
     *
     * @param resource The NetworkOperation containing the network upgrade specification.
     * @param name The name of the changeset to check, which can be one of: - Constants.TPL_VAR_RECORD_STREAM_UPLOADER -
     *     Constants.TPL_VAR_RECORD_STREAM_SIDECAR_FILES_UPLOADER - Constants.TPL_VAR_EVENT_STREAM_UPLOADER -
     *     Constants.TPL_VAR_BACKUP_UPLOADER
     * @return True if the changeset exists and is not null, otherwise false
     */
    public static boolean hasStreamUploaderChangeset(NetworkOperation resource, String name) {
        final StreamUploaderChangeset changeset = findStreamUploaderChangeset(resource, name);
        return Objects.nonNull(changeset) && Objects.nonNull(changeset.getSoftwareVersionSource());
    }

    /**
     * Converts a container name to its corresponding stream uploader type string.
     *
     * @param containerName The name of the container, which can be one of: - Constants.TPL_VAR_BACKUP_UPLOADER -
     *     Constants.TPL_VAR_EVENT_STREAM_UPLOADER - Constants.TPL_VAR_RECORD_STREAM_SIDECAR_FILES_UPLOADER -
     *     Constants.TPL_VAR_RECORD_STREAM_UPLOADER
     * @return The corresponding stream uploader type string
     * @throws IllegalStateException if the container name is unrecognized.
     */
    public static String toStreamUploaderType(String containerName) {
        return switch (containerName) {
            case Constants.TPL_VAR_BACKUP_UPLOADER -> "BackupUploader";
            case Constants.TPL_VAR_EVENT_STREAM_UPLOADER -> "EventsStreamUploader";
            case Constants.TPL_VAR_RECORD_STREAM_SIDECAR_FILES_UPLOADER -> "RecordStreamSidecarUploader";
            case Constants.TPL_VAR_RECORD_STREAM_UPLOADER -> "RecordStreamUploader";
            default -> throw new IllegalStateException("Unrecognized stream uploader container name: " + containerName);
        };
    }

    /**
     * Checks if the NetworkOperation has an OtelCollector changeset.
     *
     * @param resource The NetworkOperation to check.
     * @return True if the OtelCollector changeset exists, otherwise false
     */
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
