// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.processor;

import io.fabric8.kubernetes.api.model.KubernetesResource;
import java.time.Duration;

public record ProcessorResult(KubernetesResource status, Duration retryDelay, long maxAttempts) {
    public static final long INFINITE_RETRY = -1; // negative value means infinite retries

    public ProcessorResult(KubernetesResource status) {
        this(status, Duration.ofMillis(100), INFINITE_RETRY);
    }

    public static ProcessorResult noRetry(KubernetesResource status) {
        return new ProcessorResult(status, Duration.ZERO, 0);
    }
}
