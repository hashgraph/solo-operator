// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.processor;

import io.fabric8.kubernetes.api.model.KubernetesResource;
import java.time.Duration;

/**
 * The ProcessorResult class represents the result of a processor operation, encapsulating the status object, reconcile
 * delay, processing time, and any deferred processor.
 *
 * @param <S> The type of the Kubernetes resource status.
 */
public class ProcessorResult<S extends KubernetesResource> {
    private final S status;
    private final Duration reconcileDelay;
    private final Duration processingTime;
    private final Processor<?, ?, ?> deferred;

    /**
     * Constructs a new ProcessorResult object with the specified status, reconcile delay, processing time, and deferred
     * processor.
     *
     * @param status The status object of the processor result.
     * @param reconcileDelay The duration to wait before reconciling the resource.
     * @param processingTime The duration taken for the processing operation.
     * @param deferred The deferred processor instance.
     */
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

    /**
     * Constructs a new ProcessorResult object with the specified status and default reconcile delay, processing time,
     * and deferred processor.
     *
     * @param status The status object of the processor result.
     */
    public ProcessorResult(final S status) {
        this.status = status;
        this.reconcileDelay = Duration.ofSeconds(1);
        this.processingTime = Duration.ofSeconds(10);
        this.deferred = null;
    }

    /**
     * Returns the status object of the processor result.
     *
     * @return The status object
     */
    public S getStatus() {
        return status;
    }

    /**
     * Returns the duration to wait before reconciling the resource.
     *
     * @return The reconcile delay duration
     */
    public Duration getReconcileDelay() {
        return reconcileDelay;
    }

    /**
     * Returns the duration taken for the processing operation.
     *
     * @return The processing time duration
     */
    public Duration getProcessingTime() {
        return processingTime;
    }

    /**
     * Returns the deferred processor instance.
     *
     * @return The deferred processor
     */
    public Processor<?, ?, ?> getDeferred() {
        return deferred;
    }
}
