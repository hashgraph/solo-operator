// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps;

import static com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.common.VerifyLayerHashes.updateStatus;
import static com.hedera.solo.operator.reconciler.networkoperation.utils.ConsensusPodSidecarUtil.toStreamUploaderType;

import com.hedera.solo.operator.library.processor.ProcessorException;
import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.model.commonspec.StreamUploaderChangeset;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.common.StreamUploaderVerifier;
import com.hedera.solo.operator.reconciler.networkoperation.utils.ConsensusPodSidecarUtil;
import com.hedera.solo.operator.reconciler.networkoperation.utils.NetworkOperationStepUtil;
import com.hedera.solo.operator.reconciler.networkoperation.utils.StepConditionType;
import io.fabric8.kubernetes.api.model.Condition;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import java.lang.invoke.MethodHandles;
import java.util.List;
import java.util.Objects;
import org.jboss.logging.Logger;

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
    public static final String CONDITION_TYPE = "ImageVerified";
    public static final String SUCCESS_STATUS_MESSAGE =
            "Image verification succeeded for '%s'. Selected for deployment: %s";
    public static final String FAIL_STATUS_MESSAGE = "Image verification failed for '%s'";
    private static final Logger log = Logger.getLogger(MethodHandles.lookup().lookupClass());
    private final String containerName;

    public VerifyStreamUploaderChangesetStep(String name) {
        this.containerName = name;
    }

    @Override
    public boolean isSuccessful(NetworkOperation resource, NetworkOperationStatus status) {
        return isDone(status, containerName);
    }

    @Override
    public boolean shouldSkip(NetworkOperation resource, NetworkOperationStatus status) {
        if (!ConsensusPodSidecarUtil.hasStreamUploaderChangeset(resource, this.containerName)) {
            return true;
        }

        final StepConditionType conditionType = new StepConditionType(CONDITION_TYPE, containerName);
        return status.getPhase().equals(NetworkOperationStatus.Phase.Failed)
                || NetworkOperationStepUtil.isSuccessful(status, conditionType);
    }

    @Override
    public NetworkOperationStatus run(
            NetworkOperation resource, NetworkOperationStatus status, Context<NetworkOperation> context)
            throws ProcessorException {

        try {
            Objects.requireNonNull(resource, "NetworkOperation resource is required");
            Objects.requireNonNull(resource.getSpec(), "NetworkOperationSpec spec is required");
            Objects.requireNonNull(resource.getSpec().getNetworkUpgrade(), "NetworkUpgrade networkUpgrade is required");
            Objects.requireNonNull(status, "NetworkOperationStatus is required");
            Objects.requireNonNull(status.getPhase(), "NetworkOperationStatus is required");
            Objects.requireNonNull(context, "Context<NetworkOperation> is required");

            final StreamUploaderChangeset changeset =
                    ConsensusPodSidecarUtil.findStreamUploaderChangeset(resource, this.containerName);
            if (Objects.isNull(changeset) || Objects.isNull(changeset.getSoftwareVersionSource())) {
                if (!ConsensusPodSidecarUtil.hasStreamUploaderChangeset(resource, this.containerName)) {
                    log.debugf(
                            "Ignore checking '%s' image as it is not specified in network operation: %s/%s",
                            this.containerName,
                            resource.getMetadata().getNamespace(),
                            resource.getMetadata().getName());
                    return status;
                }
            }

            log.infof(
                    "Verifying '%s' image for network operation: %s/%s [ %s ]",
                    toStreamUploaderType(this.containerName),
                    resource.getMetadata().getNamespace(),
                    resource.getMetadata().getName(),
                    resource.getSpec().getNetworkUpgrade().getWorkflowStep());

            final NetworkOperationStatus updated = updateStatus(
                    resource,
                    status,
                    context.getClient(),
                    changeset.getSoftwareVersionSource(),
                    this::createCondition,
                    StreamUploaderVerifier.getter(this.containerName),
                    StreamUploaderVerifier.setter(this.containerName));

            if (!isDone(updated, containerName)) {
                log.warnf(
                        "Failed to verify '%s' image for network operation: %s/%s [ %s ]",
                        toStreamUploaderType(this.containerName),
                        resource.getMetadata().getNamespace(),
                        resource.getMetadata().getName(),
                        resource.getSpec().getNetworkUpgrade().getWorkflowStep());

                return NetworkOperationStepUtil.prepareStatus(updated, List.of(), NetworkOperationStatus.Phase.Failed);
            }

            log.infof(
                    "Successfully verified '%s' image for network operation: %s/%s [ %s ]",
                    toStreamUploaderType(this.containerName),
                    resource.getMetadata().getNamespace(),
                    resource.getMetadata().getName(),
                    resource.getSpec().getNetworkUpgrade().getWorkflowStep());

            return NetworkOperationStepUtil.prepareStatus(updated, List.of());
        } catch (Throwable t) {
            log.errorf(t, FAIL_STATUS_MESSAGE, t.getMessage());
            final StepConditionType conditionType = new StepConditionType(CONDITION_TYPE, containerName);
            return NetworkOperationStepUtil.prepareFailed(resource, status, t.getMessage(), conditionType);
        }
    }

    Condition createCondition(final NetworkOperationStatus status) {
        final StepConditionType conditionType = new StepConditionType(CONDITION_TYPE, this.containerName);
        return StreamUploaderVerifier.prepareCondition(
                status, this.containerName, SUCCESS_STATUS_MESSAGE, FAIL_STATUS_MESSAGE, conditionType);
    }

    public static boolean isDone(NetworkOperationStatus status, String containerName) {
        final StepConditionType conditionType = new StepConditionType(CONDITION_TYPE, containerName);
        return NetworkOperationStepUtil.isSuccessful(status, conditionType);
    }
}
