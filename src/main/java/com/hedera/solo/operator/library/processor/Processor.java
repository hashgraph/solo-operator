// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.processor;

import io.fabric8.kubernetes.api.model.HasMetadata;
import io.fabric8.kubernetes.api.model.KubernetesResource;
import io.javaoperatorsdk.operator.api.reconciler.Context;

/**
 * Processor encapsulates the data and logic to execute a single step of business process.
 *
 * @param <T> the type of the resource
 * @param <S> the type of the resource status
 * @param <R> the type of the result
 */
public interface Processor<T extends HasMetadata, S extends KubernetesResource, R extends ProcessorResult<S>> {
    /**
     * Run the business logic for the given resource and context. Each processor can run a specific task at a time,
     * where task has one or more steps.
     *
     * @param resource the resource to process
     * @param context the context
     * @return the result of the specified type R
     * @throws ProcessorException if processing the resource fails
     */
    R process(T resource, Context<T> context);
}
