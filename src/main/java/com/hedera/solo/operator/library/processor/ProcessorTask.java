// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.processor;

import io.fabric8.kubernetes.client.CustomResource;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import java.util.List;

/**
 * ProcessorTask encapsulates the data and logic to execute a single step of business process.
 *
 * @param <R> the type of the custom resource
 * @param <S> the type of the custom resource status
 */
public interface ProcessorTask<R extends CustomResource<?, S>, S> {
    /**
     * Return the list of steps to execute
     *
     * @return the list of steps
     * @throws ProcessorException if getting the steps fails
     */
    List<ProcessorStep<R, S>> steps() throws ProcessorException;

    /**
     * Execute the task
     *
     * @return the desired custom resource status
     * @throws ProcessorException if executing the task fails
     */
    default S execute(final R resource, Context<R> context) throws ProcessorException {
        List<ProcessorStep<R, S>> steps = steps();
        S current = resource.getStatus();
        for (ProcessorStep<R, S> step : steps) {
            current = step.run(resource, current, context);
        }
        return current;
    }
}
