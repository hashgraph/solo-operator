// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.processor;

import io.fabric8.kubernetes.api.model.HasMetadata;
import io.javaoperatorsdk.operator.api.reconciler.Context;

/** Processor executes a business process with the given resource object and a list of steps */
public interface Processor<R extends HasMetadata> {
    /**
     * Execute the business process with the given resource object and context
     *
     * @param resource the custom resource object
     * @param context the context
     * @return the desired custom resource object after the business process has been executed
     * @throws ProcessorException if executing the business process fails
     */
    R process(R resource, Context<R> context) throws ProcessorException;
}
