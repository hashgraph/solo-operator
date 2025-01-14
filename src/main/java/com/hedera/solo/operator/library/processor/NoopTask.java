// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.processor;

import io.fabric8.kubernetes.api.model.KubernetesResource;
import io.fabric8.kubernetes.client.CustomResource;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import java.time.Duration;
import java.util.List;

/** A no-op task that does nothing. */
public class NoopTask<T extends CustomResource<?, S>, S extends KubernetesResource>
        extends AbstractProcessorTask<T, S> {
    @Override
    protected List<ProcessorStep<T, S>> steps() throws ProcessorException {
        return List.of(new NoopStep<>());
    }

    @Override
    protected Duration computeReconcileDelay(T resource, S status) throws ProcessorException {
        return Duration.ZERO;
    }

    @Override
    protected Processor<?, ?, ?> computeDeferredProcessor(T resource, Context<T> context) throws ProcessorException {
        return null;
    }
}
