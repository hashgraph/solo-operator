// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.sequencer;

import io.fabric8.kubernetes.api.model.HasMetadata;
import java.time.Duration;

/**
 * Represents a response from the sequencer, containing details about the reconcile delay and scheduling decisions.
 *
 * @param reconcileDelay The duration to delay the reconciliation of the resource.
 * @param firstInQueue Indicates if this resource is the first in the queue for processing.
 * @param shouldReschedule Indicates if the resource should be rescheduled for processing.
 * @param <T> The type of Kubernetes resource metadata this response is associated with.
 */
public record SequencerResponse<T extends HasMetadata>(
        Duration reconcileDelay, boolean firstInQueue, boolean shouldReschedule) {}
