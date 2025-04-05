// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.render;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;

import com.hedera.solo.operator.library.builder.BuilderFunction;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.model.v1alpha1.OrbitBuilder;
import com.hedera.solo.operator.model.v1alpha1.networkoperationspec.networkupgrade.ConfigurationFiles;
import com.hedera.solo.operator.model.v1alpha1.orbitspec.Consensus;
import java.util.HashSet;
import java.util.Set;

public class NowFrozenOrbit {
    final NetworkOperation networkOperation;

    public NowFrozenOrbit(final NetworkOperation networkOperation) {
        this.networkOperation = networkOperation;
    }

    public Orbit update(final Orbit orbit) {
        if (networkOperation.getSpec().getNetworkUpgrade() == null) {
            return orbit;
        }
        Set<Long> internal = new HashSet<>(networkOperation.getStatus().getUpgradedNodes());
        return edit(orbit)
                .with(configFiles(orbit))
                .with(builder -> builder.editSpec()
                        .editConsensus()
                        .editGenesis()
                        .editAddressBook()
                        .withExternalAddresses(networkOperation.getSpec().getNetworkUpgrade().getAddressBook().stream()
                                .filter(entry -> !internal.contains(entry.getNodeId()))
                                .toList()))
                .build();
    }

    BuilderFunction<OrbitBuilder> configFiles(final Orbit orbit) {
        ConfigurationFiles config =
                networkOperation.getSpec().getNetworkUpgrade().getConfigurationFiles();
        Consensus current = orbit.getSpec().getConsensus();
        return builder -> config == null
                ? builder
                : builder.editSpec()
                        .editConsensus()
                        .withApplicationConfiguration(
                                or(config.getApplicationProperties(), current.getApplicationConfiguration()))
                        .withGenesisProperties(or(config.getGenesisProperties(), current.getGenesisProperties()))
                        .withNetworkSettings(or(config.getNetworkSettings(), current.getNetworkSettings()))
                        .withPermissionConfiguration(
                                or(config.getApiPermissions(), current.getPermissionConfiguration()));
    }

    <T> T or(final T a, final T b) {
        return a != null ? a : b;
    }
}
