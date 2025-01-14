// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.processor;

import io.fabric8.kubernetes.client.CustomResource;
import io.javaoperatorsdk.operator.api.reconciler.Context;

/** A no-op step that does nothing. */
public class NoopStep<T extends CustomResource<?, S>, S> implements ProcessorStep<T, S> {
    @Override
    public S run(T resource, S status, Context<T> context) {
        return status;
    }
}
