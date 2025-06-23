// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.check;

import io.smallrye.config.ConfigMapping;
import io.smallrye.config.WithDefault;

/** Configuration interface for network check task settings. */
@ConfigMapping(prefix = "solo.operator.network-operation.task.check")
public interface NetworkCheckTaskConfig {
    /**
     * Gets the maximum number of retries allowed for a network check task.
     *
     * @return The maximum number of retries as an integer
     */
    @WithDefault("150")
    int maxRetries();

    /**
     * Gets the interval between retries in seconds for a network check task.
     *
     * @return The retry interval as a long value
     */
    @WithDefault("2")
    long retryInterval();
}
