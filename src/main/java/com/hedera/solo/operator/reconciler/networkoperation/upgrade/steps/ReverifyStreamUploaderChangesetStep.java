// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps;

import static com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.VerifyLayerHashes.updateStatus;
import static com.hedera.solo.operator.reconciler.networkoperation.upgrade.utils.StreamUploaderUtil.computeConditionType;
import static com.hedera.solo.operator.reconciler.networkoperation.upgrade.utils.StreamUploaderUtil.toStreamUploaderType;

import com.hedera.solo.operator.library.processor.ProcessorException;
import com.hedera.solo.operator.library.processor.ProcessorStep;
import com.hedera.solo.operator.model.commonspec.StreamUploaderChangeset;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.reconciler.common.ConditionStatus;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.utils.StreamUploaderUtil;
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
public class ReverifyStreamUploaderChangesetStep extends StreamUploaderVerifier
        implements ProcessorStep<NetworkOperation, NetworkOperationStatus> {

    public static final String CONDITION_TYPE = "ImageReverification";
    private static final Logger log = Logger.getLogger(MethodHandles.lookup().lookupClass());

    public ReverifyStreamUploaderChangesetStep(String name) {
        super(name);
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
        if (Objects.isNull(changeset)
                || Objects.isNull(changeset.getSoftwareVersionSource())
                || isDone(status, this.containerName)) {
            return status; // no-op
        }

        if (status.getPhase().equals(NetworkOperationStatus.Phase.Failed) || isDone(status, this.containerName)) {
            return status;
        }

        log.infof(
                "Verifying '%s' image verification for network operation: %s/%s",
                toStreamUploaderType(this.containerName),
                resource.getMetadata().getNamespace(),
                resource.getMetadata().getName());

        return updateStatus(
                resource,
                status,
                context.getClient(),
                changeset.getSoftwareVersionSource(),
                this::createCondition,
                getter(),
                setter());
    }

    Condition createCondition(final NetworkOperationStatus status) {
        String type = computeConditionType(CONDITION_TYPE, containerName);
        return this.addCondition(status, type);
    }

    public static boolean isDone(NetworkOperationStatus status, String containerName) {
        if (status == null || status.getConditions() == null) {
            return false;
        }

        final String type = computeConditionType(CONDITION_TYPE, containerName);
        return status.getConditions().stream()
                .anyMatch(c -> c.getType().equals(type) && c.getStatus().equals(ConditionStatus.True));
    }
}
