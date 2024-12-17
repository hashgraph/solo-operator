// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.processor;

import io.fabric8.kubernetes.api.model.HasMetadata;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import java.util.List;
import java.util.Objects;

public abstract class AbstractProcessor<R extends HasMetadata> implements Processor<R> {
    protected static final String SUCCESS_MESSAGE = "SUCCEEDED: %s completed";
    protected static final String FAILURE_MESSAGE = "FAILED: %s failed: %s";

    /**
     * Get processor name
     *
     * @return the name of the processor
     */
    public abstract String getProcessorName();

    protected abstract R addSuccessCondition(R resource, Context<R> context);

    protected abstract R addFailureCondition(R resource, Context<R> context, List<ProcessorException> exceptions);

    /**
     * Prepare the steps of the processor
     *
     * @param resource the custom resource object
     * @param context the context
     * @return a list of processor steps to be executed
     * @throws ProcessorException if preparing the steps fails
     */
    public abstract List<ProcessorStep<R>> prepareSteps(R resource, Context<R> context) throws ProcessorException;

    /**
     * Default implementation of the process method
     *
     * @param resource the custom resource object
     * @param context the context
     * @return the desired object of type R after the business process has been executed
     * @throws ProcessorException if executing the business process fails
     */
    @Override
    public R process(R resource, Context<R> context) throws ProcessorException {
        List<ProcessorStep<R>> steps = prepareSteps(resource, context);
        validate(resource, context, steps);
        return executeSteps(resource, context, steps);
    }

    protected void validate(R resource, Context<R> context, List<ProcessorStep<R>> processorSteps)
            throws ProcessorException {
        Objects.requireNonNull(resource, "Primary object must not be null");
        Objects.requireNonNull(context, "Context must not be null");
        Objects.requireNonNull(processorSteps, "ProcessorSteps must not be null");
        if (processorSteps.isEmpty()) {
            throw new IllegalArgumentException("ProcessorSteps must not be empty");
        }
    }

    /**
     * Execute the processor steps in sequence and return the desired resource object
     *
     * @param resource the custom resource object
     * @param context the context
     * @param processorSteps list of processor steps to be run in sequence
     * @return the desired resource object after all steps have been executed
     * @throws ProcessorException if executing the steps fails
     */
    protected R executeSteps(R resource, Context<R> context, List<ProcessorStep<R>> processorSteps)
            throws ProcessorException {
        R current = resource;
        try {
            for (ProcessorStep<R> step : processorSteps) {
                current = step.execute(current, context);
            }
        } catch (ProcessorException e) {
            return this.addFailureCondition(current, context, List.of(e));
        }

        return this.addSuccessCondition(current, context);
    }
}
