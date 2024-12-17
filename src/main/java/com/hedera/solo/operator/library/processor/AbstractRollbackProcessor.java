// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.processor;

import io.fabric8.kubernetes.api.model.HasMetadata;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import java.util.ArrayList;
import java.util.List;

/**
 * AbstractRollbackProcessor is an abstract class that provides a method to execute a list of processor steps in
 * sequence and rollback if any of the steps fail
 *
 * @param <R> type of custom resource object
 */
public abstract class AbstractRollbackProcessor<R extends HasMetadata> extends AbstractProcessor<R> {
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

        List<ProcessorStep<R>> executed = new ArrayList<>();
        R current = resource;
        try {
            for (ProcessorStep<R> step : processorSteps) {
                current = step.execute(current, context);
                executed.add(step);
            }
        } catch (ProcessorException e) {
            final List<ProcessorException> exceptions = new ArrayList<>();
            exceptions.add(e);
            try {
                for (ProcessorStep<R> step : executed.reversed()) { // rollback should happen in reverse order
                    current = step.undo(current, context);
                    executed.add(step);
                }
            } catch (ProcessorException e2) {
                exceptions.add(e2);
            }

            return this.addFailureCondition(current, context, exceptions);
        }

        return this.addSuccessCondition(current, context);
    }
}
