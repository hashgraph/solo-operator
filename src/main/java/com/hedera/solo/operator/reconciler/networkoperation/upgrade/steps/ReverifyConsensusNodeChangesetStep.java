// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps;

import static com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.VerifyLayerHashes.updateStatus;

import com.hedera.solo.operator.library.processor.ProcessorException;
import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.model.v1alpha1.networkoperationstatus.SoftwareSourceStatus;
import com.hedera.solo.operator.reconciler.common.ConditionStatus;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import java.lang.invoke.MethodHandles;
import java.util.Objects;
import org.jboss.logging.Logger;

/**
 * ReverifyConsensusNodeChangesetStep re-verifies the consensus node changeset and selects a new image if the original
 * selection is no longer available.
 *
 * <p>Refer to
 * {@link com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.VerifyConsensusNodeChangesetStep} to
 * understand the original image source selection process.
 *
 * <p>The {@link NetworkOperationStatus} contains the image source status in the form of
 * {@link com.hedera.solo.operator.model.commonspec.ImageRepositoryStatus} in the
 * 'NetworkOperationStatus.softwareSourceStatus.consensusNode' field. The selected image source is marked as "Valid" and
 * "selectedForDeployment" is set to true.
 *
 * <p>This step performs the following business logic:
 *
 * <ol>
 *   <li>Check that the selected registry still has the image and that the hashes match (e.g., use a HEAD request). Use
 *       the manifestHash value in {@link com.hedera.solo.operator.model.commonspec.ImageRepositoryStatus} to fetch the
 *       selected manifest and then verify the image layers exist in the registry.
 *       <ol>
 *         <li>If the registry no longer has a valid image or the manifest is no longer available, look up other sources
 *             in sequence and pick the first valid one. Check {@link VerifyConsensusNodeChangesetStep} for the image
 *             source verification process. Add conditions to the status field to indicate the validation status.
 *             <ul>
 *               <li>Update the NetworkOperationStatus with this valid registry information as
 *                   {@link com.hedera.solo.operator.model.commonspec.ImageRepositoryStatus}.
 *               <li>Update the old one with "Invalid" status.
 *             </ul>
 *         <li>If no valid registry is found, emit an error and abort. Mark the NetworkOperation as failed using a
 *             {@link java.util.concurrent.locks.Condition}
 *         <li>Finally, add a condition to the status field explaining the validation status if it is successful.
 *       </ol>
 * </ol>
 */
public class ReverifyConsensusNodeChangesetStep extends ConsensusVerifier
        implements ProcessorStep<NetworkOperation, NetworkOperationStatus> {
    private static final Logger log = Logger.getLogger(MethodHandles.lookup().lookupClass());

    @Override
    public NetworkOperationStatus run(
            NetworkOperation resource, NetworkOperationStatus status, Context<NetworkOperation> context)
            throws ProcessorException {
        Objects.requireNonNull(resource, "NetworkOperation resource is required");
        Objects.requireNonNull(status, "NetworkOperationStatus status is required");
        Objects.requireNonNull(status.getPhase(), "NetworkOperationStatus phase is required");
        Objects.requireNonNull(context, "Context<NetworkOperation> context is required");

        if (status.getPhase().equals(NetworkOperationStatus.Phase.Failed) || isDone(status)) {
            return status;
        }

        log.infof(
                "Verifying consensus node image for network operation: %s/%s",
                resource.getMetadata().getNamespace(), resource.getMetadata().getName());
        return updateStatus(
                resource,
                status,
                context.getClient(),
                getSoftwareVersionSource(resource),
                this::createCondition,
                SoftwareSourceStatus::getConsensusNode,
                SoftwareSourceStatus::setConsensusNode);
    }

    public static boolean isDone(NetworkOperationStatus status) {
        if (status == null || status.getConditions() == null) {
            return false;
        }

        return status.getConditions().stream()
                .anyMatch(c -> c.getType().equals(ConsensusVerifier.CONDITION_TYPE)
                        && c.getStatus().equals(ConditionStatus.True));
    }
}
