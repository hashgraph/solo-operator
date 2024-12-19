// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.processor;

import io.fabric8.kubernetes.client.CustomResource;
import io.javaoperatorsdk.operator.api.reconciler.Context;

/**
 * ProcessorStep encapsulates the data and logic to execute a single step of a task.
 *
 * @param <T> the type of the custom resource
 * @param <S> the type of the custom resource status
 */
public interface ProcessorStep<T extends CustomResource<?, S>, S> {
    /**
     * Execute the step
     *
     * @return the desired custom resource status
     * @throws ProcessorException if executing the step fails
     */
    S run(final T resource, final S status, Context<T> context);
}
