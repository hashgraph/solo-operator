// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.processor;

import io.fabric8.kubernetes.api.model.KubernetesResource;
import io.fabric8.kubernetes.client.CustomResource;
import io.javaoperatorsdk.operator.api.reconciler.Context;

/**
 * ProcessorTask encapsulates the data and logic to execute a single task of the processor It also defines the steps to
 * execute for the task
 *
 * @param <T> the type of the custom resource
 * @param <S> the type of the custom resource status
 */
public interface ProcessorTask<T extends CustomResource<?, S>, S extends KubernetesResource> {
    /**
     * Execute the task
     *
     * @return the desired custom resource status
     * @throws ProcessorException if executing the task fails
     */
    ProcessorResult<S> execute(final T resource, Context<T> context);
}
