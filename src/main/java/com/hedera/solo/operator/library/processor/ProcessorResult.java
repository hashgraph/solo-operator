// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.processor;

import io.fabric8.kubernetes.api.model.KubernetesResource;
import java.time.Duration;

public class ProcessorResult<S extends KubernetesResource> {
    private final S status;
    private final Duration reconcileDelay;

    public ProcessorResult(final S status, final Duration reconcileDelay) {
        this.status = status;
        this.reconcileDelay = reconcileDelay;
    }

    public S getStatus() {
        return status;
    }

    public Duration getReconcileDelay() {
        return reconcileDelay;
    }
}
