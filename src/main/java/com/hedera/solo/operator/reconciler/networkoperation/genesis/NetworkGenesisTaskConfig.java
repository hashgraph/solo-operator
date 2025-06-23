// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.genesis;

import io.smallrye.config.ConfigMapping;
import io.smallrye.config.WithDefault;

/** Configuration interface for the network operation task related to genesis creation. */
@ConfigMapping(prefix = "solo.operator.network-operation.task.genesis")
public interface NetworkGenesisTaskConfig {

    /**
     * Gets the maximum number of retry attempts for the genesis creation task.
     *
     * @return The maximum number of retries
     */
    @WithDefault("150")
    int maxRetries();

    /**
     * Gets the interval between retry attempts in seconds.
     *
     * @return The interval between retries
     */
    @WithDefault("2")
    long retryInterval();
}
