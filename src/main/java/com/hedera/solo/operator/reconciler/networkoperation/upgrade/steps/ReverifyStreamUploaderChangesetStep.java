// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps;

import static com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.common.VerifyLayerHashes.updateStatus;
import static com.hedera.solo.operator.reconciler.networkoperation.utils.StreamUploaderUtil.computeConditionType;
import static com.hedera.solo.operator.reconciler.networkoperation.utils.StreamUploaderUtil.toStreamUploaderType;

import com.hedera.solo.operator.library.processor.ProcessorException;
import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.model.commonspec.StreamUploaderChangeset;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.common.StreamUploaderVerifier;
import com.hedera.solo.operator.reconciler.networkoperation.utils.NetworkOperationStepUtil;
import com.hedera.solo.operator.reconciler.networkoperation.utils.StreamUploaderUtil;
import io.fabric8.kubernetes.api.model.Condition;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import java.lang.invoke.MethodHandles;
import java.util.Objects;
import org.jboss.logging.Logger;

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
public class ReverifyStreamUploaderChangesetStep implements ProcessorStep<NetworkOperation, NetworkOperationStatus> {
    public static final String CONDITION_TYPE = "ImageReVerified: StreamUploader";
    public static final String SUCCESS_STATUS_MESSAGE =
            "Image re-verification succeeded for '%s'. Selected for deployment: %s";
    public static final String FAIL_STATUS_MESSAGE = "Image re-verification failed for '%s'";

    private static final Logger log = Logger.getLogger(MethodHandles.lookup().lookupClass());
    private final String containerName;

    public ReverifyStreamUploaderChangesetStep(String containerName) {
        this.containerName = containerName;
    }

    @Override
    public boolean isSuccessful(NetworkOperation resource, NetworkOperationStatus status) {
        return isDone(status, containerName);
    }

    @Override
    public boolean shouldSkip(NetworkOperation resource, NetworkOperationStatus status) {
        final String conditionType = computeConditionType(CONDITION_TYPE, containerName);
        return status.getPhase().equals(NetworkOperationStatus.Phase.Failed)
                || NetworkOperationStepUtil.isSuccessful(status, conditionType);
    }

    @Override
    public NetworkOperationStatus run(
            NetworkOperation resource, NetworkOperationStatus status, Context<NetworkOperation> context)
            throws ProcessorException {
        try {
            Objects.requireNonNull(resource, "NetworkOperation resource is required");
            Objects.requireNonNull(status, "NetworkOperationStatus status is required");
            Objects.requireNonNull(status.getPhase(), "NetworkOperationStatus phase is required");
            Objects.requireNonNull(context, "Context<NetworkOperation> context is required");

            final StreamUploaderChangeset changeset = StreamUploaderUtil.findChangeset(resource, this.containerName);
            if (Objects.isNull(changeset) || Objects.isNull(changeset.getSoftwareVersionSource())) {
                log.debugf(
                        "Ignore checking '%s' image as it is not specified in network operation: %s/%s",
                        this.containerName,
                        resource.getMetadata().getNamespace(),
                        resource.getMetadata().getName());
                return status; // no-op
            }

            log.infof(
                    "Re-verifying '%s' image for network operation: %s/%s",
                    toStreamUploaderType(this.containerName),
                    resource.getMetadata().getNamespace(),
                    resource.getMetadata().getName());

            final NetworkOperationStatus updated = updateStatus(
                    resource,
                    status,
                    context.getClient(),
                    changeset.getSoftwareVersionSource(),
                    this::createCondition,
                    StreamUploaderVerifier.getter(this.containerName),
                    StreamUploaderVerifier.setter(this.containerName));

            return NetworkOperationStepUtil.checkLastCondition(updated);
        } catch (Throwable t) {
            log.errorf(t, FAIL_STATUS_MESSAGE, t.getMessage());
            return NetworkOperationStepUtil.prepareFailed(resource, status, t.getMessage(), CONDITION_TYPE);
        }
    }

    Condition createCondition(final NetworkOperationStatus status) {
        String conditionType = computeConditionType(CONDITION_TYPE, containerName);
        return StreamUploaderVerifier.prepareCondition(
                status, this.containerName, SUCCESS_STATUS_MESSAGE, FAIL_STATUS_MESSAGE, conditionType);
    }

    public static boolean isDone(NetworkOperationStatus status, String containerName) {
        final String conditionType = computeConditionType(CONDITION_TYPE, containerName);
        return NetworkOperationStepUtil.isSuccessful(status, conditionType);
    }
}
