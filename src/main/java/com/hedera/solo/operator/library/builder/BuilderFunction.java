// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.builder;

import io.fabric8.kubernetes.api.builder.Builder;

@FunctionalInterface
public interface BuilderFunction<B extends Builder<?>> {
    Object applyChecked(final B builder) throws Exception;

    default Object apply(final B builder) {
        try {
            return applyChecked(builder);
        } catch (Exception e) {
            throw e instanceof RuntimeException r ? r : new RuntimeException(e);
        }
    }
}
