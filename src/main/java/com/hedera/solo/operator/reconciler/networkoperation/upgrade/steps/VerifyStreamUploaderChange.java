// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;

import com.hedera.solo.operator.library.processor.AbstractStep;
import com.hedera.solo.operator.model.commonspec.SoftwareVersionStatusBuilder;
import com.hedera.solo.operator.model.commonspec.StreamUploaderChangeset;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.reconciler.common.Constants;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import java.util.Objects;

public class VerifyStreamUploaderChange extends AbstractStep<NetworkOperation> {
    private final String containerName;

    public VerifyStreamUploaderChange(String name) {
        this.containerName = name;
    }

    public static StreamUploaderChangeset getStreamUploaderChangeset(NetworkOperation primary, String name) {
        return switch (name) {
            case Constants.TPL_VAR_RECORD_STREAM_UPLOADER -> primary.getSpec()
                    .getNetworkUpgrade()
                    .getChangesets()
                    .getRecordStreamUploaderChangeset();
            case Constants.TPL_VAR_RECORD_STREAM_SIDECAR_FILES_UPLOADER -> primary.getSpec()
                    .getNetworkUpgrade()
                    .getChangesets()
                    .getRecordStreamSidecarUploaderChangeset();
            case Constants.TPL_VAR_EVENT_STREAM_UPLOADER -> primary.getSpec()
                    .getNetworkUpgrade()
                    .getChangesets()
                    .getEventsStreamUploaderChangeset();
            case Constants.TPL_VAR_BACKUP_UPLOADER -> primary.getSpec()
                    .getNetworkUpgrade()
                    .getChangesets()
                    .getBackupUploaderChangeset();
            default -> null;
        };
    }

    @Override
    public NetworkOperation execute(NetworkOperation resource, Context<NetworkOperation> context) {
        final StreamUploaderChangeset changeset = getStreamUploaderChangeset(resource, this.containerName);
        if (Objects.isNull(changeset)) {
            return resource; // no-op
        }

        final String manifestHash = "NOT_SET"; // FIXME validate using image coordinate validator

        final SoftwareVersionStatusBuilder sb = new SoftwareVersionStatusBuilder()
                .withName(containerName)
                .withRepository(changeset.getSoftwareVersion().getRepository())
                .withTag(changeset.getSoftwareVersion().getTag())
                .withManifestHash(manifestHash)
                .withSelectedForDeployment(true);

        return edit(resource)
                .with(status -> status.editStatus()
                        .editOrNewSoftwareSourceStatus()
                        .addToStreamUploader(sb.build())
                        .endSoftwareSourceStatus())
                .build();
    }
}
