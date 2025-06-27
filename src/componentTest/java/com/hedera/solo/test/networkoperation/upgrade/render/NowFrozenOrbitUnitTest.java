// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.test.networkoperation.upgrade.render;

import static com.hedera.solo.operator.library.generic.Nullable.orElseGet;
import static org.assertj.core.api.Assertions.assertThat;

import com.hedera.solo.operator.model.commonspec.AddressBookEntry;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationSpec;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.model.v1alpha1.OrbitSpec;
import com.hedera.solo.operator.model.v1alpha1.networkoperationspec.NetworkUpgrade;
import com.hedera.solo.operator.model.v1alpha1.networkoperationspec.networkupgrade.ConfigurationFiles;
import com.hedera.solo.operator.model.v1alpha1.orbitspec.Consensus;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.render.NowFrozenOrbit;
import com.hedera.solo.operator.test.fixtures.fuzz.BeanFactory;
import java.util.ArrayList;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Set;
import org.junit.jupiter.api.RepeatedTest;

/**
 * This class contains unit tests for the NowFrozenOrbit class, which is responsible for updating an Orbit based on a
 * NetworkOperation specification.
 */
public class NowFrozenOrbitUnitTest {
    BeanFactory factory = beanFactory();

    /**
     * Initializes a new BeanFactory instance.
     *
     * @return A new BeanFactory instance
     */
    BeanFactory beanFactory() {
        return new BeanFactory();
    }

    /** Repeatedly tests the update method of NowFrozenOrbit with random data to ensure it behaves as expected. */
    @RepeatedTest(10)
    public void test() {
        Orbit initialOrbit = orbit();
        NetworkOperation networkOperation = networkOperation(initialOrbit);
        Orbit updatedOrbit = update(networkOperation, initialOrbit);
        ConfigurationFiles files = orElseGet(
                () -> networkOperation.getSpec().getNetworkUpgrade().getConfigurationFiles(), ConfigurationFiles::new);
        Consensus initialConsensus = initialOrbit.getSpec().getConsensus();
        Consensus updatedConsensus = updatedOrbit.getSpec().getConsensus();
        match(
                updatedConsensus.getApplicationConfiguration(),
                files.getApplicationProperties(),
                initialConsensus.getApplicationConfiguration());
        match(
                updatedConsensus.getGenesisProperties(),
                files.getGenesisProperties(),
                initialConsensus.getGenesisProperties());
        match(updatedConsensus.getNetworkSettings(), files.getNetworkSettings(), initialConsensus.getNetworkSettings());
        match(
                updatedConsensus.getPermissionConfiguration(),
                files.getPermissionConfiguration(),
                initialConsensus.getPermissionConfiguration());
        match(networkOperation, updatedOrbit);
    }

    /**
     * Asserts that the network operation and orbit specifications are consistent.
     *
     * @param networkOperation The NetworkOperation instance to check against the Orbit specification.
     * @param orbit The Orbit instance to compare with the NetworkOperation specification.
     */
    void match(final NetworkOperation networkOperation, final Orbit orbit) {
        List<AddressBookEntry> addressBook =
                new ArrayList<>(networkOperation.getSpec().getNetworkUpgrade().getAddressBook());
        Set<Long> internal = new HashSet<>(networkOperation.getStatus().getUpgradedNodes());
        addressBook.removeIf(entry -> internal.contains(entry.getNodeId()));
        assertThat(addressBook)
                .isEqualTo(orbit.getSpec()
                        .getConsensus()
                        .getGenesis()
                        .getAddressBook()
                        .getExternalAddresses());
    }

    /**
     * Updates an Orbit instance based on the specification provided in a NetworkOperation.
     *
     * @param networkOperation The NetworkOperation containing the update specifications for the Orbit.
     * @param initial The initial Orbit to be updated.
     * @return The updated Orbit
     */
    Orbit update(final NetworkOperation networkOperation, final Orbit initial) {
        NowFrozenOrbit updater = new NowFrozenOrbit(networkOperation);
        return updater.update(initial);
    }

    /**
     * Compares two maps of string values and asserts that they are equal.
     *
     * @param updated The map to be compared against the update map.
     * @param update The expected map for the update operation. If null, the initial map is used for comparison.
     * @param initial The initial map used for comparison if the update map is null.
     */
    void match(final Map<String, String> updated, final Map<String, String> update, final Map<String, String> initial) {
        assertThat(updated).isEqualTo(update != null ? update : initial);
    }

    /**
     * Initializes a NetworkOperation instance with default specifications based on an Orbit.
     *
     * @param orbit The Orbit to base the NetworkOperation specification on.
     * @return A new NetworkOperation instance initialized with default specifications
     */
    NetworkOperation networkOperation(final Orbit orbit) {
        NetworkOperation networkOperation = factory.nonNull(NetworkOperation.class);
        if (networkOperation.getSpec() == null) {
            networkOperation.setSpec(factory.nonNull(NetworkOperationSpec.class));
        }
        networkOperation.getSpec().setOrbit(orbit.getMetadata().getName());
        if (networkOperation.getSpec().getNetworkUpgrade() == null) {
            networkOperation.getSpec().setNetworkUpgrade(factory.nonNull(NetworkUpgrade.class));
        }
        if (networkOperation.getSpec().getNetworkUpgrade().getAddressBook() == null) {
            networkOperation
                    .getSpec()
                    .getNetworkUpgrade()
                    .setAddressBook(factory.nonNulls(AddressBookEntry.class)
                            .limit(factory.random().nextInt(8))
                            .toList());
        } else {
            networkOperation
                    .getSpec()
                    .getNetworkUpgrade()
                    .setAddressBook(networkOperation.getSpec().getNetworkUpgrade().getAddressBook().stream()
                            .filter(Objects::nonNull)
                            .toList());
        }
        if (networkOperation.getStatus() == null) {
            networkOperation.setStatus(factory.nonNull(NetworkOperationStatus.class));
        }
        if (networkOperation.getStatus().getUpgradedNodes() == null) {
            networkOperation
                    .getStatus()
                    .setUpgradedNodes(factory.nonNulls(Long.class)
                            .limit(factory.random().nextInt(8))
                            .toList());
        }
        if (!networkOperation.getStatus().getUpgradedNodes().isEmpty()) {
            if (!networkOperation.getSpec().getNetworkUpgrade().getAddressBook().isEmpty()) {
                networkOperation
                        .getSpec()
                        .getNetworkUpgrade()
                        .getAddressBook()
                        .getFirst()
                        .setNodeId(
                                networkOperation.getStatus().getUpgradedNodes().getFirst());
            }
        }
        return networkOperation;
    }

    /**
     * Initializes an Orbit instance with default specifications.
     *
     * @return A new Orbit instance initialized with default specifications
     */
    Orbit orbit() {
        Orbit orbit = factory.nonNull(Orbit.class);
        if (orbit.getSpec() == null) {
            orbit.setSpec(factory.nonNull(OrbitSpec.class));
        }
        if (orbit.getSpec().getConsensus() == null) {
            orbit.getSpec().setConsensus(factory.nonNull(Consensus.class));
        }
        return orbit;
    }
}
