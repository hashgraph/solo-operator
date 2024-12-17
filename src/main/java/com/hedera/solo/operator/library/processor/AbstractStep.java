// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.processor;

import io.fabric8.kubernetes.api.model.HasMetadata;
import io.javaoperatorsdk.operator.api.reconciler.Context;

public abstract class AbstractStep<R extends HasMetadata> implements ProcessorStep<R> {

    @Override
    public R undo(R resource, Context<R> context) throws ProcessorException {
        return resource; // by default no-op
    }
}
