// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.unit;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static org.assertj.core.api.Assertions.assertThat;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitialization;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitializationFluent;
import com.hedera.solo.operator.reconciler.networkinitialization.resources.NetworkInitializationConsensusCapsule;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import org.junit.jupiter.api.Test;

public class NetworkInitializationConsensusCapsuleTest {
    @Test
    void test() {
        Map<String, ConsensusCapsule> map = new HashMap<>(Map.of(
                "ignored",
                edit(new ConsensusCapsule())
                        .with(metadata -> metadata.editMetadata().withName("name"))
                        .build()));
        // null status
        NetworkInitialization primary = new NetworkInitialization();
        assertThat(NetworkInitializationConsensusCapsule.getSecondaryResources(primary, map))
                .isNotEmpty();
        // null capsules
        primary = edit(primary).with(NetworkInitializationFluent::withNewStatus).build();
        assertThat(NetworkInitializationConsensusCapsule.getSecondaryResources(primary, map))
                .isNotEmpty();
        // capsule in status
        primary = edit(primary)
                .with(status -> status.editStatus().withConsensusCapsules(List.of("name")))
                .build();
        assertThat(NetworkInitializationConsensusCapsule.getSecondaryResources(primary, map))
                .isNotEmpty();
        // capsule not in status
        primary = edit(primary)
                .with(status -> status.editStatus().withConsensusCapsules(List.of("not-found")))
                .build();
        assertThat(NetworkInitializationConsensusCapsule.getSecondaryResources(primary, map))
                .isEmpty();
    }
}
