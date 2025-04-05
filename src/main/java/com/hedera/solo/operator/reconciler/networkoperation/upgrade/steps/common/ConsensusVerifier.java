// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.common;

import static com.hedera.solo.operator.library.time.Timestamp.timestamp;
import static com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.common.VerifyLayerHashes.selectedImage;

import com.hedera.solo.operator.model.commonspec.ConsensusNodeChangeset;
import com.hedera.solo.operator.model.commonspec.SoftwareVersionSource;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.reconciler.common.ConditionStatus;
import io.fabric8.kubernetes.api.model.Condition;
import java.util.Objects;

public final class ConsensusVerifier {
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

    public static Condition createCondition(
            final NetworkOperationStatus status,
            final String successMessageTemplate,
            final String failMessageTemplate,
            final String conditionType) {
        String image = selectedImage(status.getSoftwareSourceStatus().getConsensusNode());
        String timestamp = timestamp();
        return image == null
                ? new Condition(null, failMessageTemplate, null, null, ConditionStatus.False, conditionType)
                : new Condition(
                        timestamp,
                        successMessageTemplate.formatted(image),
                        null,
                        null,
                        ConditionStatus.True,
                        conditionType);
    }
}
