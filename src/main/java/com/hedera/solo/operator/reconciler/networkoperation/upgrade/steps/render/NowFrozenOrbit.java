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

/**
 * The NowFrozenOrbit class is responsible for updating an Orbit based on the specification of a NetworkOperation during
 * network upgrade.
 */
public class NowFrozenOrbit {
    final NetworkOperation networkOperation;

    /**
     * Constructs a new NowFrozenOrbit with the given NetworkOperation.
     *
     * @param networkOperation The NetworkOperation containing the specifications for the network upgrade.
     */
    public NowFrozenOrbit(final NetworkOperation networkOperation) {
        this.networkOperation = networkOperation;
    }

    /**
     * Updates the Orbit based on the NetworkOperation specification.
     *
     * @param orbit The original Orbit to be updated.
     * @return The updated Orbit with changes applied from the NetworkOperation
     */
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

    /**
     * A function that applies configuration files from the NetworkOperation to the Orbit builder.
     *
     * @param orbit The original Orbit instance used for reference during configuration file application.
     * @return A BuilderFunction that modifies the OrbitBuilder based on the ConfigurationFiles in the NetworkOperation
     */
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
                                or(config.getPermissionConfiguration(), current.getPermissionConfiguration()));
    }

    /**
     * Helper method to return the first non-null value between two objects of type T.
     *
     * @param a The first object to check.
     * @param b The second object to check if the first is null.
     * @param <T> The type of the objects.
     * @return The first object if it's not null, otherwise the second object
     */
    <T> T or(final T a, final T b) {
        return a != null ? a : b;
    }
}
