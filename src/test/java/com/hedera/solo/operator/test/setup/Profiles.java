// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.setup;

import io.quarkus.test.junit.QuarkusTestProfile;
import java.util.Map;
import java.util.Set;

public class Profiles {
    public static final String TAG_NO_CLUSTER = "no-cluster";

    public static class NoCluster implements QuarkusTestProfile {
        @Override
        public Map<String, String> getConfigOverrides() {
            return Map.of("quarkus.kubernetes-client.devservices.enabled", "false");
        }

        @Override
        public Set<String> tags() {
            return Set.of(TAG_NO_CLUSTER);
        }
    }
}
