// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.operator.library.time.Timestamp.timestamp;
import static com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.common.VerifyLayerHashes.updateStatus;
import static com.hedera.solo.operator.reconciler.networkoperation.utils.StreamUploaderUtil.computeConditionType;
import static com.hedera.solo.operator.reconciler.networkoperation.utils.StreamUploaderUtil.toStreamUploaderType;

import com.hedera.solo.operator.library.processor.ProcessorException;
import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.model.commonspec.StreamUploaderChangeset;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.reconciler.common.ConditionStatus;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.common.StreamUploaderVerifier;
import com.hedera.solo.operator.reconciler.networkoperation.utils.NetworkOperationStepUtil;
import com.hedera.solo.operator.reconciler.networkoperation.utils.StreamUploaderUtil;
import io.fabric8.kubernetes.api.model.Condition;
import io.fabric8.kubernetes.api.model.ConditionBuilder;
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
    static final String SUCCESS_STATUS_MESSAGE =
            "Image re-verification succeeded for '%s'. Selected for deployment: %s";
    static final String FAIL_STATUS_MESSAGE = "Image re-verification failed for '%s'";

    private static final Logger log = Logger.getLogger(MethodHandles.lookup().lookupClass());
    private final String containerName;

    public ReverifyStreamUploaderChangesetStep(String containerName) {
        this.containerName = containerName;
    }

    @Override
    public NetworkOperationStatus run(
            NetworkOperation resource, NetworkOperationStatus status, Context<NetworkOperation> context)
            throws ProcessorException {
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

        if (status.getPhase().equals(NetworkOperationStatus.Phase.Failed) || isDone(status, this.containerName)) {
            log.debugf(
                    "Skipping re-verification of '%s' image for network operation: %s/%s [ Phase: %s, Done: %s ]",
                    this.containerName,
                    resource.getMetadata().getNamespace(),
                    resource.getMetadata().getName(),
                    status.getPhase(),
                    isDone(status, this.containerName));
            return status;
        }

        log.infof(
                "Re-verifying '%s' image for network operation: %s/%s",
                toStreamUploaderType(this.containerName),
                resource.getMetadata().getNamespace(),
                resource.getMetadata().getName());

        String timestamp = timestamp();
        try {
            final NetworkOperationStatus updated = updateStatus(
                    resource,
                    status,
                    context.getClient(),
                    changeset.getSoftwareVersionSource(),
                    this::createCondition,
                    StreamUploaderVerifier.getter(this.containerName),
                    StreamUploaderVerifier.setter(this.containerName));

            return NetworkOperationStepUtil.checkLastCondition(updated);
        } catch (final Exception ex) {
            String conditionType = computeConditionType(CONDITION_TYPE, containerName);
            log.errorf(
                    ex,
                    "Failed to re-verify '%s' for network operation: %s/%s",
                    toStreamUploaderType(this.containerName),
                    resource.getMetadata().getNamespace(),
                    resource.getMetadata().getName());

            return edit(status)
                    .with(s -> s.addToConditions(new ConditionBuilder()
                                    .withObservedGeneration(
                                            resource.getMetadata().getGeneration())
                                    .withType(conditionType)
                                    .withStatus(ConditionStatus.False)
                                    .withLastTransitionTime(timestamp)
                                    .withMessage("%s: %s".formatted(FAIL_STATUS_MESSAGE, ex.getMessage()))
                                    .build())
                            .withPhase(NetworkOperationStatus.Phase.Failed))
                    .build();
        }
    }

    Condition createCondition(final NetworkOperationStatus status) {
        String conditionType = computeConditionType(CONDITION_TYPE, containerName);
        return StreamUploaderVerifier.prepareCondition(
                status, this.containerName, SUCCESS_STATUS_MESSAGE, FAIL_STATUS_MESSAGE, conditionType);
    }

    public static boolean isDone(NetworkOperationStatus status, String containerName) {
        final String conditionType = computeConditionType(CONDITION_TYPE, containerName);
        return NetworkOperationStepUtil.isDone(status, conditionType);
    }
}
