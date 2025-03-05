// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.sequencer;

import io.fabric8.kubernetes.api.model.HasMetadata;
import java.time.Duration;

public record SequencerRequest<T extends HasMetadata>(T resource, Duration reconcileDelay, Duration processingTime) {
    public SequencerRequest(T resource) {
        this(resource, Sequencer.DEFAULT_RECONCILIATION_DELAY, Sequencer.DEFAULT_ESTIMATED_PROCESSING_TIME);
    }

    public SequencerRequest(T resource, Duration reconcileDelay) {
        this(resource, reconcileDelay, Sequencer.DEFAULT_ESTIMATED_PROCESSING_TIME);
    }

    public static <T extends HasMetadata> SequencerRequest<T> of(
            T resource, Duration reconcileDelay, Duration processingTime) {
        return new SequencerRequest<>(resource, reconcileDelay, processingTime);
    }
}
