// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.common;

import static com.hedera.solo.operator.library.time.Timestamp.timestamp;
import static com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.common.VerifyLayerHashes.selectedImage;

import com.hedera.solo.operator.model.commonspec.ConsensusNodeChangeset;
import com.hedera.solo.operator.model.commonspec.SoftwareVersionSource;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.reconciler.common.ConditionStatus;
import com.hedera.solo.operator.reconciler.networkoperation.utils.StepConditionType;
import io.fabric8.kubernetes.api.model.Condition;
import java.util.Objects;

/**
 * The {@code ConsensusVerifier} class provides utility methods to retrieve and manipulate software version source from
 * a NetworkOperation object, as well as creating conditions for the network operation status.
 */
public final class ConsensusVerifier {
    /** Private constructor to prevent instantiation of this utility class. */
    private ConsensusVerifier() {}

    /**
     * Retrieves the SoftwareVersionSource from the consensus-node changeset within the NetworkOperation specification.
     *
     * @param resource The NetworkOperation object containing the spec for the network upgrade.
     * @return The SoftwareVersionSource associated with the consensus-node changeset
     */
    public static SoftwareVersionSource getSoftwareVersionSource(final NetworkOperation resource) {
        Objects.requireNonNull(resource, "NetworkOperation must not be null");
        Objects.requireNonNull(resource.getSpec(), "NetworkOperation spec must not be null");
        Objects.requireNonNull(resource.getSpec().getNetworkUpgrade(), "Network upgrade spec must not be null");
        Objects.requireNonNull(resource.getSpec().getNetworkUpgrade().getChangesets(), "Changesets must not be null");
        final ConsensusNodeChangeset changeset =
                resource.getSpec().getNetworkUpgrade().getChangesets().getConsensusNodeChangeset();
        Objects.requireNonNull(changeset, "consensus-node changeset must not be null");
        Objects.requireNonNull(changeset.getSoftwareVersionSource(), "SoftwareVersionSource must not be null");

        return changeset.getSoftwareVersionSource();
    }

    /**
     * Creates a condition object for the NetworkOperationStatus, based on whether the selected image is available or
     * not.
     *
     * @param status The current status of the NetworkOperation.
     * @param successMessageTemplate A template string for the success message.
     * @param failMessageTemplate A template string for the failure message.
     * @param conditionType An instance of StepConditionType representing the type and reason of the condition.
     * @return A Condition object, either successful or failed based on the availability of the selected image
     */
    public static Condition createCondition(
            final NetworkOperationStatus status,
            final String successMessageTemplate,
            final String failMessageTemplate,
            final StepConditionType conditionType) {
        String image = selectedImage(status.getSoftwareSourceStatus().getConsensusNode());
        String timestamp = timestamp();
        return image == null
                ? new Condition(
                        timestamp,
                        failMessageTemplate,
                        null,
                        conditionType.reason(),
                        ConditionStatus.False,
                        conditionType.type())
                : new Condition(
                        timestamp,
                        successMessageTemplate.formatted(image),
                        null,
                        conditionType.reason(),
                        ConditionStatus.True,
                        conditionType.type());
    }
}
