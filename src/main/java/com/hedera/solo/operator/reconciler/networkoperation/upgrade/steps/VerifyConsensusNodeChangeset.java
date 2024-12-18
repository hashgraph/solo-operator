// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.operator.reconciler.common.Constants.TPL_VAR_CONSENSUS_NODE;

import com.hedera.solo.operator.library.processor.ProcessorException;
import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.model.commonspec.ConsensusNodeChangeset;
import com.hedera.solo.operator.model.commonspec.SoftwareVersionSource;
import com.hedera.solo.operator.model.commonspec.SoftwareVersionStatusBuilder;
import com.hedera.solo.operator.model.commonspec.softwareversionsource.Images;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import java.util.Objects;

public class VerifyConsensusNodeChangeset implements ProcessorStep<NetworkOperation, NetworkOperationStatus> {
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
        if (changeset.getSoftwareVersionSource().isEmpty()) {
            throw new ProcessorException("SoftwareVersionSource must not be empty");
        }

        final SoftwareVersionStatusBuilder sb = new SoftwareVersionStatusBuilder();
        boolean selectedForDeployment = false;
        for (SoftwareVersionSource ss : changeset.getSoftwareVersionSource()) {
            for (Images image : ss.getImages()) {
                String manifestHash = "NOT_SET"; // FIXME validate using image coordinate validator
                sb.withName(TPL_VAR_CONSENSUS_NODE)
                        .withRepository(image.getRepository())
                        .withTag(image.getTag())
                        .withManifestHash(manifestHash)
                        .withSelectedForDeployment(true);
                break; // break after first valid image??
            }
        }

        return edit(status)
                .with(r -> r.editOrNewSoftwareSourceStatus()
                        .addToConsensusNode(sb.build())
                        .endSoftwareSourceStatus())
                .build();
    }
}
