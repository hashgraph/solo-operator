package com.hedera.solo.operator.library.builder;

import io.fabric8.kubernetes.api.builder.Builder;

public interface BuilderFunction<B extends Builder<?>> {
    Object applyChecked(B builder) throws Exception;
    default Object apply(B builder) {
        try {
            return applyChecked(builder);
        } catch (Exception e) {
            throw new RuntimeException(e);
        }
    }
}
