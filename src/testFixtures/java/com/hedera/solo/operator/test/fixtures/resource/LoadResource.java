// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.fixtures.resource;

import static com.hedera.solo.operator.test.fixtures.resource.ConvertResource.properties;
import static com.hedera.solo.operator.test.fixtures.resource.ConvertResource.settings;
import static com.hedera.solo.operator.test.fixtures.resource.SystemResource.getSystemResourceAsStringUnchecked;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsuleBuilder;
import com.hedera.solo.operator.model.v1alpha1.OrbitBuilder;

public class LoadResource {
    public static ConsensusCapsuleBuilder consensusCapsuleBuilderWithResources() {
        return consensusCapsuleBuilderWithResources(new ConsensusCapsuleBuilder());
    }

    public static ConsensusCapsuleBuilder consensusCapsuleBuilderWithResources(ConsensusCapsuleBuilder builder) {
        return builder.editSpec()
                .withLog4j2Xml(getSystemResourceAsStringUnchecked("import/hedera/mainnet/log4j2.xml"))
                .endSpec();
    }

    public static OrbitBuilder orbitBuilderWithResources() {
        return orbitBuilderWithResources(new OrbitBuilder());
    }

    public static OrbitBuilder orbitBuilderWithResources(OrbitBuilder builder) {
        return builder.editSpec()
                .editConsensus()
                .withApplicationConfiguration(properties("import/hedera/mainnet/application.properties"))
                .withNetworkSettings(settings("import/hedera/mainnet/settings.txt"))
                .withPermissionConfiguration(properties("import/hedera/mainnet/api-permission.properties"))
                .endConsensus()
                .endSpec();
    }
}
