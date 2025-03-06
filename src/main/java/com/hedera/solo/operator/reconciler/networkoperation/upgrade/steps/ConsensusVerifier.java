// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps;

import static com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.VerifyLayerHashes.selectedImage;

import com.hedera.solo.operator.model.commonspec.ConsensusNodeChangeset;
import com.hedera.solo.operator.model.commonspec.SoftwareVersionSource;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.reconciler.common.ConditionStatus;
import io.fabric8.kubernetes.api.model.Condition;
import java.util.Objects;

public class ConsensusVerifier {
    public static final String CONDITION_TYPE = "ImageVerified: ConsensusNode";
    static final String SUCCESS_STATUS_MESSAGE = "Consensus image verification succeeded. Selected for deployment: %s";
    static final String FAIL_STATUS_MESSAGE = "Consensus image verification failed";

    SoftwareVersionSource getSoftwareVersionSource(final NetworkOperation resource) {
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

    Condition createCondition(final NetworkOperationStatus status) {
        String image = selectedImage(status.getSoftwareSourceStatus().getConsensusNode());
        return image == null
                ? new Condition(null, FAIL_STATUS_MESSAGE, null, null, ConditionStatus.False, CONDITION_TYPE)
                : new Condition(
                        null,
                        SUCCESS_STATUS_MESSAGE.formatted(image),
                        null,
                        null,
                        ConditionStatus.True,
                        CONDITION_TYPE);
    }
}
