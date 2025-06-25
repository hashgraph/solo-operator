// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.sequencer;

import io.fabric8.kubernetes.api.model.HasMetadata;
import java.time.Duration;

/**
 * Represents a request to the sequencer for processing a Kubernetes resource with optional delays for reconciliation
 * and processing.
 *
 * @param resource The Kubernetes resource to be processed.
 * @param reconcileDelay The duration to delay reconciliation of the resource.
 * @param processingTime The estimated time required to process the resource.
 * @param <T> The type of Kubernetes resource being processed.
 */
public record SequencerRequest<T extends HasMetadata>(T resource, Duration reconcileDelay, Duration processingTime) {

    /**
     * Constructs a new {@code SequencerRequest} with default reconciliation and processing times.
     *
     * @param resource The Kubernetes resource to be processed.
     */
    public SequencerRequest(T resource) {
        this(resource, Sequencer.DEFAULT_RECONCILIATION_DELAY, Sequencer.DEFAULT_ESTIMATED_PROCESSING_TIME);
    }

    /**
     * Constructs a new {@code SequencerRequest} with a specified reconciliation delay and default processing time.
     *
     * @param resource The Kubernetes resource to be processed.
     * @param reconcileDelay The duration for the reconciliation delay.
     */
    public SequencerRequest(T resource, Duration reconcileDelay) {
        this(resource, reconcileDelay, Sequencer.DEFAULT_ESTIMATED_PROCESSING_TIME);
    }

    /**
     * Creates a new {@code SequencerRequest} with the specified reconciliation delay and processing time.
     *
     * @param <T> The type of Kubernetes resource being processed.
     * @param resource The Kubernetes resource to be processed.
     * @param reconcileDelay The duration for the reconciliation delay.
     * @param processingTime The estimated processing time for the resource.
     * @return A new {@code SequencerRequest} instance
     */
    public static <T extends HasMetadata> SequencerRequest<T> of(
            T resource, Duration reconcileDelay, Duration processingTime) {
        return new SequencerRequest<>(resource, reconcileDelay, processingTime);
    }
}
