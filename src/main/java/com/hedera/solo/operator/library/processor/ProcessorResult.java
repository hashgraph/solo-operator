// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.processor;

import io.fabric8.kubernetes.api.model.KubernetesResource;
import java.time.Duration;

public class ProcessorResult<S extends KubernetesResource> {
    private final S status;
    private final Duration reconcileDelay;
    private final Duration processingTime;
    private final Processor<?, ?, ?> deferred;

    public ProcessorResult(
            final S status,
            final Duration reconcileDelay,
            final Duration processingTime,
            final Processor<?, ?, ?> deferred) {
        this.status = status;
        this.reconcileDelay = reconcileDelay;
        this.processingTime = processingTime;
        this.deferred = deferred;
    }

    public ProcessorResult(final S status) {
        this.status = status;
        this.reconcileDelay = Duration.ofSeconds(1);
        this.processingTime = Duration.ofSeconds(10);
        this.deferred = null;
    }

    public S getStatus() {
        return status;
    }

    public Duration getReconcileDelay() {
        return reconcileDelay;
    }

    public Duration getProcessingTime() {
        return processingTime;
    }

    public Processor<?, ?, ?> getDeferred() {
        return deferred;
    }
}
