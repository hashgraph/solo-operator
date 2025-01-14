// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps;

import com.hedera.solo.operator.library.processor.ProcessorException;
import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.model.commonspec.StreamUploaderChangeset;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.utils.StreamUploaderUtil;
import io.javaoperatorsdk.operator.api.reconciler.Context;

/**
 * VerifyStreamUploaderChangesetStep verifies the changeset for StreamUploader
 *
 * <p>The {@link StreamUploaderChangeset} includes the repository sources
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
 * 'NetworkOperationStatus.softwareSourceStatus.streamUploader' field and adds conditions indicating the success or
 * failure of the verification status of the source in 'NetworkOperationStatus.conditions'. The conditions are added to
 * the status in the order of the sources for audit purposes.
 *
 * <p>For each {@link com.hedera.solo.operator.model.commonspec.ImageRepository} in the sources, this step performs the
 * same verification logic as it does for consensus node image sources. Only difference is that the changeset is for
 * relevant stream-uploader container specified by the "containerName" property.
 *
 * <p>If valid image source is not found (the image or layer does not exist or doesn't match with hashes in CR), add a
 * condition to the status indicating the failure and failure reason.
 *
 * <p>If a valid image source is found, add a condition to the status indicating the success and the selected source URL
 */
public class VerifyStreamUploaderChangesetStep implements ProcessorStep<NetworkOperation, NetworkOperationStatus> {
    private final String containerName;
    public static final String SUCCESS_STATUS_MESSAGE =
            "Stream uploader image verification succeeded for '%s'. Selected for deployment: %s";
    public static final String FAIL_STATUS_MESSAGE = "Stream uploader image verification failed for '%s'";
    public static final String IMAGE_VERIFICATION_SUCCESS_MESSAGE = "Image verification succeeded for '%s': %s";
    public static final String IMAGE_VERIFICATION_FAIL_MESSAGE = "Image verification failed for '%s': %s";

    public VerifyStreamUploaderChangesetStep(String name) {
        this.containerName = name;
    }

    @Override
    public NetworkOperationStatus run(
            NetworkOperation resource, NetworkOperationStatus status, Context<NetworkOperation> context)
            throws ProcessorException {
        final StreamUploaderChangeset changeset = StreamUploaderUtil.findChangeset(resource, this.containerName);

        // FIXME update status
        return status;
    }
}
