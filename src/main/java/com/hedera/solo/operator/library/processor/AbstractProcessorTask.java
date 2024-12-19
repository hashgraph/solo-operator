// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.processor;

import io.fabric8.kubernetes.api.model.KubernetesResource;
import io.fabric8.kubernetes.client.CustomResource;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import java.time.Duration;
import java.util.List;

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
     * Execute the task
     *
     * <p>
     *
     * @return the desired custom resource status
     * @throws ProcessorException if executing the task fails
     */
    public ProcessorResult<S> execute(final T resource, final Context<T> context) throws ProcessorException {
        List<ProcessorStep<T, S>> steps = steps();
        S current = resource.getStatus();
        for (ProcessorStep<T, S> step : steps) {
            current = step.run(resource, current, context);
        }
        return new ProcessorResult<>(current, computeReconcileDelay(resource, current));
    }
}
