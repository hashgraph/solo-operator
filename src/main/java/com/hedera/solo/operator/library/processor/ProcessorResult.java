// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.processor;

import io.fabric8.kubernetes.api.model.KubernetesResource;
import java.time.Duration;

public class ProcessorResult<S extends KubernetesResource> {
    private final S status;
    private final Duration reconcileDelay;
    private final Processor<?, ?, ?> deferred;

    public ProcessorResult(final S status, final Duration reconcileDelay, Processor<?, ?, ?> deferred) {
        this.status = status;
        this.reconcileDelay = reconcileDelay;
        this.deferred = deferred;
    }

    public ProcessorResult(final S status) {
        this.status = status;
        this.reconcileDelay = Duration.ZERO;
        this.deferred = null;
    }

    public S getStatus() {
        return status;
    }

    public Duration getReconcileDelay() {
        return reconcileDelay;
    }

    public Processor<?, ?, ?> getDeferred() {
        return deferred;
    }
}
