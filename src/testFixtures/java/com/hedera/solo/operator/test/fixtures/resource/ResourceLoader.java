// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.fixtures.resource;

import static com.hedera.solo.operator.test.fixtures.resource.ResourceConverter.asMap;
import static com.hedera.solo.operator.test.fixtures.resource.SystemResource.getSystemResourceAsString;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsuleBuilder;
import com.hedera.solo.operator.model.v1alpha1.OrbitBuilder;

/**
 * The ResourceLoader class provides utility methods to load resources for building ConsensusCapsule and Orbit objects.
 */
public class ResourceLoader {
    /** Private constructor to prevent instantiation of this utility class. */
    private ResourceLoader() {}

    /**
     * Configures a ConsensusCapsuleBuilder with system resources.
     *
     * @param builder the ConsensusCapsuleBuilder instance to configure
     * @return the configured ConsensusCapsuleBuilder
     */
    public static ConsensusCapsuleBuilder withResources(final ConsensusCapsuleBuilder builder) {
        return builder.editSpec()
                .withLog4j2Xml(getSystemResourceAsString("import/hedera/mainnet/log4j2.xml"))
                .endSpec();
    }

    /**
     * Configures an OrbitBuilder with system resources.
     *
     * @param builder the OrbitBuilder instance to configure
     * @return the configured OrbitBuilder
     */
    public static OrbitBuilder withResources(final OrbitBuilder builder) {
        return builder.editSpec()
                .editConsensus()
                .withApplicationConfiguration(asMap("import/hedera/mainnet/application.properties", '='))
                .withNetworkSettings(asMap("import/hedera/mainnet/settings.txt", ','))
                .withPermissionConfiguration(asMap("import/hedera/mainnet/api-permission.properties", '='))
                .endConsensus()
                .endSpec();
    }
}
