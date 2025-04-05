// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.processor;

import io.fabric8.kubernetes.client.CustomResource;
import io.javaoperatorsdk.operator.api.reconciler.Context;

/**
 * ProcessorStep encapsulates the data and logic to execute a single step of a task.
 *
 * <p>A ProcessorStep is a single unit of work that can be executed by a Processor.
 *
 * <p>A ProcessorStep must not throw exceptions. If a ProcessorStep fails, it should handle errors and return the status
 * with appropriate metadata.
 *
 * @param <T> the type of the custom resource
 * @param <S> the type of the custom resource status
 */
public interface ProcessorStep<T extends CustomResource<?, S>, S> {
    /**
     * Execute the step
     *
     * <p>This method must never throw exception.
     *
     * @return updated custom resource status
     */
    S run(final T resource, final S status, Context<T> context);

    /**
     * Check if the step is successful
     *
     * @param status the custom resource status
     * @return true if the step is successful, false otherwise
     */
    boolean isSuccessful(final T resource, final S status);

    /**
     * Check if the step should be skipped
     *
     * <p>Step's run method must not skip processing based on response from this method as it is left to be handled by
     * the {@link AbstractProcessorTask} and {@link RetryStep} to control the flow.
     *
     * @param resource the custom resource
     * @param status the custom resource status
     * @return true if the step should be skipped, false otherwise
     */
    boolean shouldSkip(final T resource, final S status);
}
