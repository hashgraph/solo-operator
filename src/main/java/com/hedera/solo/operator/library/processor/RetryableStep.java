// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.processor;

import io.fabric8.kubernetes.client.CustomResource;
import io.javaoperatorsdk.operator.api.reconciler.Context;

/**
 * RetryableStep denotes a step that can be retried multiple times
 *
 * <p> It needs to be wrapped in {@code Retryable> wrapper class for execution.
 * <p>
 *
 * @param <T> the type of the custom resource
 * @param <S> the type of the custom resource status
 */
public interface RetryableStep<T extends CustomResource<?, S>, S> extends ProcessorStep<T, S> {
    /**
     * Handle the final exception that occurred after maximum number of retries
     *
     * <p>If the error cannot be handled, the step should rethrow the exception
     *
     * @param resource the custom resource
     * @param status the custom resource status
     * @param context the context
     * @param ex the exception
     * @return the updated custom resource status
     */
    S handleRetryError(final T resource, final S status, Context<T> context, final Exception ex);
}
