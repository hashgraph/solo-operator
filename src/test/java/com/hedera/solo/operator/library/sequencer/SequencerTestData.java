// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.sequencer;

import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationBuilder;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationSpec;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;

public final class SequencerTestData {
    public static final String orbit1 = "orbit-1";
    public static final String orbit2 = "orbit-2";
    public static final NetworkOperation no1 = new NetworkOperationBuilder()
            .withNewMetadata()
            .withName("no1")
            .withUid("no1-uuid")
            .withNamespace(orbit1)
            .withCreationTimestamp("2025-01-28T00:00:00Z")
            .endMetadata()
            .withNewSpec()
            .withVersion("v2")
            .withMode(NetworkOperationSpec.Mode.NetworkGenesis)
            .endSpec()
            .withNewStatus()
            .withPhase(NetworkOperationStatus.Phase.Pending)
            .endStatus()
            .build();
    public static final NetworkOperation no2 = new NetworkOperationBuilder()
            .withNewMetadata()
            .withName("no2")
            .withUid("no2-uuid")
            .withNamespace(orbit1)
            .withCreationTimestamp("2025-01-28T00:00:01Z")
            .endMetadata()
            .withNewSpec()
            .withMode(NetworkOperationSpec.Mode.NetworkCheck)
            .withVersion("v2")
            .endSpec()
            .build();
    public static final NetworkOperation no3 = new NetworkOperationBuilder()
            .withNewMetadata()
            .withName("no3")
            .withUid("no3-uuid")
            .withNamespace(orbit1)
            .withCreationTimestamp("2025-01-28T00:00:01Z")
            .endMetadata()
            .withNewSpec()
            .withMode(NetworkOperationSpec.Mode.NetworkCheck)
            .withVersion("v2")
            .endSpec()
            .build();
    public static final NetworkOperation no4 = new NetworkOperationBuilder()
            .withNewMetadata()
            .withName("no4")
            .withUid("no4-uuid")
            .withNamespace(orbit2)
            .withCreationTimestamp("2025-01-28T00:00:01Z")
            .endMetadata()
            .withNewSpec()
            .withMode(NetworkOperationSpec.Mode.NetworkCheck)
            .withVersion("v2")
            .endSpec()
            .build();
}
