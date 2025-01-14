// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.processor;

import io.fabric8.kubernetes.api.model.KubernetesResource;
import io.fabric8.kubernetes.client.CustomResource;
import io.javaoperatorsdk.operator.api.reconciler.Context;

/**
 * A no-op processor that does nothing.
 *
 * <p>If a processor task identifies that it has nothing to do, it may defer the control to this processor. The
 * reconciler will check the ProcessorResult and look for deferred processor to execute or take necessary action to
 * avoid calling the original processor repeatedly.
 */
public class NoopProcessor<T extends CustomResource<?, S>, S extends KubernetesResource>
        implements Processor<T, S, ProcessorResult<S>> {

    private final ProcessorTask<T, S> task = new NoopTask<>();

    @Override
    public ProcessorResult<S> process(T resource, Context<T> context) {
        return task.execute(resource, context);
    }
}
