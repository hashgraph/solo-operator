// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.processor;

import io.fabric8.kubernetes.api.model.KubernetesResource;
import io.fabric8.kubernetes.client.CustomResource;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import java.time.Duration;
import java.util.List;
import java.util.Objects;

/**
 * An abstract class containing common functionality of ProcessorTask.
 *
 * @param <T> the type of the custom resource
 * @param <S> the type of the custom resource status
 */
public abstract class AbstractProcessorTask<T extends CustomResource<?, S>, S extends KubernetesResource>
        implements ProcessorTask<T, S> {
    /**
     * Return the list of steps to execute
     *
     * <p>
     *
     * @return the list of steps
     * @throws ProcessorException if getting the steps fails
     */
    protected abstract List<ProcessorStep<T, S>> steps() throws ProcessorException;

    protected abstract Duration computeReconcileDelay(final T resource, final S status) throws ProcessorException;

    /**
     * Check if the task should be deferred to another processor
     *
     * @param resource the custom resource
     * @param context the context
     * @return the processor to defer to, or null if it should not be deferred
     * @throws ProcessorException exception is wrapped in a ProcessorException
     */
    protected abstract Processor<?, ?, ?> computeDeferredProcessor(final T resource, Context<T> context)
            throws ProcessorException;

    /**
     * Execute the task
     *
     * <p>
     *
     * @return the desired custom resource status
     * @throws ProcessorException if executing the task fails
     */
    public ProcessorResult<S> execute(final T resource, final Context<T> context) throws ProcessorException {
        S current = resource.getStatus();

        // check if it should be deferred
        final Processor<?, ?, ?> deferred = computeDeferredProcessor(resource, context);
        if (Objects.nonNull(deferred)) {
            return new ProcessorResult<S>(current, computeReconcileDelay(resource, current), deferred);
        }

        // process the steps and determine the new status
        final List<ProcessorStep<T, S>> steps = steps();
        for (ProcessorStep<T, S> step : steps) {
            current = step.run(resource, current, context);
        }

        return new ProcessorResult<S>(current, computeReconcileDelay(resource, current), null);
    }
}
