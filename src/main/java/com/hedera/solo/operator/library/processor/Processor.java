// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.processor;

import io.fabric8.kubernetes.api.model.HasMetadata;
import io.javaoperatorsdk.operator.api.reconciler.Context;

/**
 * Processor encapsulates the data and logic to execute a single step of business process.
 *
 * @param <R> the type of the resource
 * @param <T> the type of the result
 */
public interface Processor<R extends HasMetadata, T> {
    /**
     * Process the resource
     *
     * @param resource the resource to process
     * @param context the context
     * @return the result of the specified type R
     * @throws ProcessorException if processing the resource fails
     */
    T process(R resource, Context<R> context) throws ProcessorException;
}
