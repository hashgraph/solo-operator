package com.hedera.solo.operator.library.builder;

import io.fabric8.kubernetes.api.builder.Builder;

public interface BuilderFunction<B extends Builder<?>> {
    Object applyChecked(final B builder) throws Exception;
    default Object apply(final B builder) {
        try {
            return applyChecked(builder);
        } catch (Exception e) {
            throw new RuntimeException(e);
        }
    }
}
