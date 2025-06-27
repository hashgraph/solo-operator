// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.test.library.k8s;

import static com.hedera.solo.operator.library.k8s.KubeletConfig.clusterDomain;
import static org.assertj.core.api.Assertions.assertThat;

import org.junit.jupiter.api.Test;

/**
 * This class contains unit tests for the KubeletConfig utility methods, specifically focusing on the cluster domain
 * configuration.
 */
public class KubeletConfigTest {
    @Test
    void testFailureHandling() {
        assertThat(clusterDomain(null)).isNull();
    }
}
