// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps;

import static com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.VerifyLayerHashes.updateStatus;

import com.hedera.solo.operator.library.processor.ProcessorException;
import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.model.commonspec.StreamUploaderChangeset;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.utils.StreamUploaderUtil;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import java.util.Objects;

/**
 * ReverifyStreamUploaderChangesetStep re-verifies the changeset for the stream uploaders and selects a new image if
 * original selection is not available.
 *
 * <p>Check {@link ReverifyConsensusNodeChangesetStep} for consensus node changeset re-verification and apply similar
 * logic for stream uploader changeset denoted by the containerName field.
 *
 * <p>The {@link NetworkOperationStatus} contains the image source status in the form of *
 * {@link com.hedera.solo.operator.model.commonspec.ImageRepositoryStatus} in the *
 * 'NetworkOperationStatus.softwareSourceStatus.streamUploader' field. The selected image source is marked as "Valid"
 * and * "selectedForDeployment" is set to true.
 *
 * <p>If the specified image is same for all stream uploader containers, then re-reverification is not required for all
 * stream uploader changesets.
 *
 * <p>The changeset can be found by passing the NetworkOperation resource and containerName to
 * {@link StreamUploaderUtil#findChangeset(NetworkOperation, String)}.
 */
public class ReverifyStreamUploaderChangesetStep extends StreamUploaderVerifier
        implements ProcessorStep<NetworkOperation, NetworkOperationStatus> {
    public ReverifyStreamUploaderChangesetStep(String name) {
        super(name);
    }

    @Override
    public NetworkOperationStatus run(
            NetworkOperation resource, NetworkOperationStatus status, Context<NetworkOperation> context)
            throws ProcessorException {

        final StreamUploaderChangeset changeset = StreamUploaderUtil.findChangeset(resource, this.containerName);

        if (Objects.isNull(changeset) || changeset.getSoftwareVersionSource() == null) {
            return status; // no-op
        }

        // FIXME: Implement the logic to reverify the changeset

        return updateStatus(
                resource,
                status,
                context.getClient(),
                changeset.getSoftwareVersionSource(),
                this::createCondition,
                getter(),
                setter());
    }
}
