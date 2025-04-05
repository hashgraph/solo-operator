// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.genesis;

import io.smallrye.config.ConfigMapping;
import io.smallrye.config.WithDefault;

@ConfigMapping(prefix = "solo.operator.network-operation.task.genesis")
public interface NetworkGenesisTaskConfig {
    @WithDefault("150")
    int maxRetries();

    @WithDefault("2")
    long retryInterval();
}
