// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.processor;

import io.fabric8.kubernetes.api.model.HasMetadata;
import io.javaoperatorsdk.operator.api.reconciler.Context;

/** ProcessorStep encapsulates the data and logic to execute a single step of business process. */
public interface ProcessorStep<P extends HasMetadata> {
    /**
     * Execute the step
     *
     * @return the desired custom resource object after executing the step
     * @throws ProcessorException if executing the step fails
     */
    P execute(P resource, Context<P> context);

    /**
     * Undo the step
     *
     * @return the desired custom resource object after undoing the step
     * @throws ProcessorException if undoing the step fails
     */
    P undo(P resource, Context<P> context) throws ProcessorException;
}
