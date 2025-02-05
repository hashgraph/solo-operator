// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps;

import static com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.VerifyLayerHashes.selectedImage;
import static com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.VerifyLayerHashes.updateStatus;

import com.hedera.solo.operator.library.processor.ProcessorException;
import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.model.commonspec.ConsensusNodeChangeset;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.model.v1alpha1.networkoperationstatus.SoftwareSourceStatus;
import com.hedera.solo.operator.reconciler.common.ConditionStatus;
import io.fabric8.kubernetes.api.model.Condition;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import java.util.Objects;

/**
 * This step is responsible for verifying the consensus node changeset and updating the status accordingly.
 *
 * <p>The {@link ConsensusNodeChangeset} includes the repository sources
 * ({@link com.hedera.solo.operator.model.commonspec.SoftwareVersionSource}) from various node operators (~32 or more).
 *
 * <p>In this step, we fetch image manifest from each of those sources, check that the layer exists as well as match
 * that in the CR and select one (randomly) from the valid sources for deployment.
 *
 * <p>We select a source randomly because it would then ensure not a single source is overloaded when all nodes are
 * trying to pull the image during deployment.
 *
 * <p>Note that the sources are assumed to be valid and put in order of preference in the CR. Rather than selecting the
 * first valid source for deployment, we select one randomly for load-balancing purposes.
 *
 * <p>During execution, the step updates the {@link NetworkOperationStatus} object with the selected source in
 * 'NetworkOperationStatus.softwareSourceStatus.consensusNode' field and adds conditions indicating the success or
 * failure of the verification status of the source in 'NetworkOperationStatus.conditions'. The conditions are added to
 * the status in the order of the sources for audit purposes.
 *
 * <p>It is important to review the following sources and objects to understand the context of this step:
 *
 * <ol>
 *   <li>'src/main/k8s/openapi/common.yml' - The common spec objects such as ImageRepository, ImageRepositoryStatus,
 *   <li>'src/main/k8s/openapi/networkoperation.yml' - The CRD object that represents the network operation
 *   <li>'src/test/resources/templates/network-operation/network-upgrade.yaml' - An example network operation CR
 *   <li>{@link NetworkOperation} - The CRD object that represents the network operation
 *   <li>{@link NetworkOperationStatus} - The status object that represents the status of the network operation
 *   <li>{@link ConsensusNodeChangeset} - The object that represents the consensus node changeset in the CR
 *   <li>{@link com.hedera.solo.operator.model.commonspec.SoftwareVersionSource} - The object that represents the
 *       software
 *   <li>{@link com.hedera.solo.operator.model.commonspec.ImageRepository} - The object that represents the image
 *       repository in the CR
 *   <li>{@link com.hedera.solo.operator.model.commonspec.ImageRepositoryStatus} - The object that represents the image
 *   <li>{@link io.fabric8.kubernetes.api.model.Condition} - The object that represents the condition in the status
 * </ol>
 *
 * <p>For each {@link com.hedera.solo.operator.model.commonspec.ImageRepository} in the sources, this step does the
 * following:
 *
 * <ol>
 *   <li>Fetch the image manifest (use imagePullSecret if specified to fetch the manifest from private registry)
 *   <li>Verify that the layer digests in the manifest match the required layer hashes in the CR for a given platform
 *       and architecture
 *   <li>If all layer digests match, verify that the layers exist in the registry (send HEAD request instead of fetching
 *       the layer)
 *   <li>If the repository is valid (all the layer exist and match the required hashes), do the following:
 *       <ol>
 *         <li>create a {@link com.hedera.solo.operator.model.commonspec.ImageRepositoryStatus} object with the
 *             repository details and save the manifest hash in the ImageRepositoryStatus.manifestHash.
 *         <li>If we haven't already selected one for deployment, flip a coin (e.g. randomly select among N, where N the
 *             total number of entries) to decide if the repository should be selected for deployment; and update the
 *             flag `selectedForDeployment`. Keep track of the selected repository so that we don't need to perform this
 *             step again and also can add a final success condition with the selected repository details.
 *         <li>Add RepositoryStatus object to the list of repository statuses in
 *             'NetworkOperationStatus.softwareSourceStatus.consensusNode'
 *         <li>add a condition {@link java.util.concurrent.locks.Condition} to the status indicating the success in
 *             'NetworkOperationStatus.conditions'
 *       </ol>
 * </ol>
 *
 * <p>If valid image source is not found (the image or layer does not exist or doesn't match with hashes in CR), add a
 * condition to the status indicating the failure and failure reason.
 *
 * <p>If a valid image source is found, add a condition to the status indicating the success and the selected source URL
 */
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
        Objects.requireNonNull(
                resource.getSpec().getNetworkUpgrade().getChangesets().getConsensusNodeChangeset(),
                "consensus-node changeset must not be null");

        return resource.getSpec().getNetworkUpgrade().getChangesets().getConsensusNodeChangeset();
    }

    @Override
    public NetworkOperationStatus run(
            NetworkOperation resource, NetworkOperationStatus status, Context<NetworkOperation> context)
            throws ProcessorException {
        Objects.requireNonNull(resource, "NetworkOperation must not be null");
        Objects.requireNonNull(context, "Context must not be null");

        final ConsensusNodeChangeset changeset = this.getConsensusNodeChangeset(resource);
        Objects.requireNonNull(changeset.getSoftwareVersionSource(), "SoftwareVersionSource must not be null");

        return updateStatus(
                resource,
                status,
                context.getClient(),
                changeset.getSoftwareVersionSource(),
                this::createCondition,
                SoftwareSourceStatus::getConsensusNode,
                SoftwareSourceStatus::setConsensusNode);
    }

    Condition createCondition(final NetworkOperationStatus status) {
        String type = getClass().getSimpleName();
        String image = selectedImage(status.getSoftwareSourceStatus().getConsensusNode());
        return image == null
                ? new Condition(null, FAIL_STATUS_MESSAGE, null, null, ConditionStatus.False, type)
                : new Condition(null, SUCCESS_STATUS_MESSAGE.formatted(image), null, null, ConditionStatus.True, type);
    }
}
