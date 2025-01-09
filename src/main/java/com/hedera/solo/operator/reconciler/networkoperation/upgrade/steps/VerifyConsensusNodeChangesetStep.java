// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps;

import com.hedera.solo.operator.library.processor.ProcessorException;
import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.model.commonspec.ConsensusNodeChangeset;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import java.util.Objects;

public class VerifyConsensusNodeChangesetStep implements ProcessorStep<NetworkOperation, NetworkOperationStatus> {
    public static final String SUCCESS_STATUS_MESSAGE =
            "Consensus image verification succeeded. Selected for deployment: %s";
    public static final String FAIL_STATUS_MESSAGE = "Consensus image verification failed";
    public static final String IMAGE_VERIFICATION_SUCCESS_MESSAGE = "Image verification succeeded: %s";
    public static final String IMAGE_VERIFICATION_FAIL_MESSAGE = "Image verification failed: %s";

    private ConsensusNodeChangeset getConsensusNodeChangeset(NetworkOperation resource) {
        Objects.requireNonNull(resource, "NetworkOperation must not be null");
        Objects.requireNonNull(resource.getSpec(), "NetworkOperation spec must not be null");
        Objects.requireNonNull(resource.getSpec().getNetworkUpgrade(), "Network upgrade spec must not be null");
        Objects.requireNonNull(resource.getSpec().getNetworkUpgrade().getChangesets(), "Changesets must not be null");

        return resource.getSpec().getNetworkUpgrade().getChangesets().getConsensusNodeChangeset();
    }

    @Override
    public NetworkOperationStatus run(
            NetworkOperation resource, NetworkOperationStatus status, Context<NetworkOperation> context)
            throws ProcessorException {
        Objects.requireNonNull(resource, "NetworkOperation must not be null");
        Objects.requireNonNull(context, "Context must not be null");

        final ConsensusNodeChangeset changeset = this.getConsensusNodeChangeset((NetworkOperation) resource);
        if (Objects.isNull(changeset)) {
            throw new ProcessorException("Changeset not found for consensus-node");
        }

        Objects.requireNonNull(changeset.getSoftwareVersionSource(), "SoftwareVersionSource must not be null");

        // FIXME update status
        return status;
    }
}
