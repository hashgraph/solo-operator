// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.fixtures.resource;

import static com.hedera.solo.operator.test.fixtures.resource.ResourceConverter.asMap;
import static com.hedera.solo.operator.test.fixtures.resource.SystemResource.getSystemResourceAsString;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsuleBuilder;
import com.hedera.solo.operator.model.v1alpha1.OrbitBuilder;

public class ResourceLoader {
    private ResourceLoader() {}

    public static ConsensusCapsuleBuilder withResources(final ConsensusCapsuleBuilder builder) {
        return builder.editSpec()
                .withLog4j2Xml(getSystemResourceAsString("import/hedera/mainnet/log4j2.xml"))
                .endSpec();
    }

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
