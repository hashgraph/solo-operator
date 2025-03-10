// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.sequencer;

import io.fabric8.kubernetes.api.model.HasMetadata;
import java.time.Duration;

public record SequencerResponse<T extends HasMetadata>(
        Duration reconcileDelay, boolean firstInQueue, boolean shouldReschedule) {}
