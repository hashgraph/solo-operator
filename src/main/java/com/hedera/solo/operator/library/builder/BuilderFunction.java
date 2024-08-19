package com.hedera.solo.operator.library.builder;

import io.fabric8.kubernetes.api.builder.Builder;

public interface ExceptionalFunction<B extends Builder<?>> {
    Object apply(B builder) throws Exception;
}
