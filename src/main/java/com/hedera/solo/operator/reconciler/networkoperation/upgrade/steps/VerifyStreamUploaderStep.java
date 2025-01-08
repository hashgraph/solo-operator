// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;

import com.hedera.solo.operator.library.processor.ProcessorException;
import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.model.commonspec.SoftwareVersionStatusBuilder;
import com.hedera.solo.operator.model.commonspec.StreamUploaderChangeset;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.utils.StreamUploaderUtil;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import java.util.Objects;

public class VerifyStreamUploaderStep implements ProcessorStep<NetworkOperation, NetworkOperationStatus> {
    private final String containerName;

    public VerifyStreamUploaderStep(String name) {
        this.containerName = name;
    }

    @Override
    public NetworkOperationStatus run(
            NetworkOperation resource, NetworkOperationStatus status, Context<NetworkOperation> context)
            throws ProcessorException {
        final StreamUploaderChangeset changeset = StreamUploaderUtil.findChangeset(resource, this.containerName);
        if (Objects.isNull(changeset)) {
            return status; // no-op
        }

        final String manifestHash = "NOT_SET"; // FIXME validate using image coordinate validator

        final SoftwareVersionStatusBuilder sb = new SoftwareVersionStatusBuilder()
                .withName(containerName)
                .withRepository(changeset.getSoftwareVersion().getRepository())
                .withTag(changeset.getSoftwareVersion().getTag())
                .withManifestHash(manifestHash)
                .withSelectedForDeployment(true);

        return edit(status)
                .with(s -> s.editOrNewSoftwareSourceStatus()
                        .addToStreamUploader(sb.build())
                        .endSoftwareSourceStatus())
                .build();
    }
}
