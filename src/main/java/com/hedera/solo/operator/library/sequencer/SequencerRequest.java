// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.sequencer;

import io.fabric8.kubernetes.api.model.HasMetadata;
import java.time.Duration;

public record SequencerRequest<T extends HasMetadata>(T resource, Duration reconcileDelay, Duration processingTime) {
    public SequencerRequest(T resource) {
        this(resource, Duration.ZERO, Duration.ZERO);
    }

    public SequencerRequest(T resource, Duration reconcileDelay) {
        this(resource, reconcileDelay, Duration.ZERO);
    }

    public static <T extends HasMetadata> SequencerRequest<T> of(
            T resource, Duration reconcileDelay, Duration processingTime) {
        return new SequencerRequest<>(resource, reconcileDelay, processingTime);
    }
}
