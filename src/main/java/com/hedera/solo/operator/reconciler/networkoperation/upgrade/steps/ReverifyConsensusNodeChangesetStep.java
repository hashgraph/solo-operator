// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps;

import static com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.common.VerifyLayerHashes.updateStatus;

import com.hedera.solo.operator.library.processor.ProcessorException;
import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.model.v1alpha1.networkoperationstatus.SoftwareSourceStatus;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.common.ConsensusVerifier;
import com.hedera.solo.operator.reconciler.networkoperation.utils.NetworkOperationStepUtil;
import io.fabric8.kubernetes.api.model.Condition;
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
public class ReverifyConsensusNodeChangesetStep implements ProcessorStep<NetworkOperation, NetworkOperationStatus> {
    public static final String CONDITION_TYPE = "ImageReVerified: ConsensusNode";
    public static final String SUCCESS_STATUS_MESSAGE =
            "Image re-verification succeeded for 'consensus-node'. Selected for deployment: %s";
    public static final String FAIL_STATUS_MESSAGE = "Image re-verification failed for 'consensus-node'";
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
            log.debugf(
                    "Skipping re-verification of 'consensus-node' image for network operation: %s/%s [ Phase: %s, Done: %s ]",
                    resource.getMetadata().getNamespace(),
                    resource.getMetadata().getName(),
                    status.getPhase(),
                    isDone(status));
            return status;
        }

        log.infof(
                "Re-verifying consensus node image for network operation: %s/%s",
                resource.getMetadata().getNamespace(), resource.getMetadata().getName());

        final NetworkOperationStatus updated = updateStatus(
                resource,
                status,
                context.getClient(),
                ConsensusVerifier.getSoftwareVersionSource(resource),
                this::createCondition,
                SoftwareSourceStatus::getConsensusNode,
                SoftwareSourceStatus::setConsensusNode);

        return NetworkOperationStepUtil.checkLastCondition(updated);
    }

    @Override
    public NetworkOperationStatus handleError(
            NetworkOperation resource, NetworkOperationStatus status, Context<NetworkOperation> context, Exception ex) {
        Objects.requireNonNull(resource, "NetworkOperation resource is required");
        Objects.requireNonNull(status, "NetworkOperation status is required");
        Objects.requireNonNull(ex, "Exception is required");

        log.errorf(
                ex,
                "Failed to re-verify consensus node image for network operation: %s/%s",
                resource.getMetadata().getNamespace(),
                resource.getMetadata().getName());

        return NetworkOperationStepUtil.prepareFailed(resource, status, ex.getMessage(), CONDITION_TYPE);
    }

    private Condition createCondition(final NetworkOperationStatus status) {
        return ConsensusVerifier.createCondition(status, SUCCESS_STATUS_MESSAGE, FAIL_STATUS_MESSAGE, CONDITION_TYPE);
    }

    public static boolean isDone(NetworkOperationStatus status) {
        return NetworkOperationStepUtil.isDone(status, CONDITION_TYPE);
    }
}
