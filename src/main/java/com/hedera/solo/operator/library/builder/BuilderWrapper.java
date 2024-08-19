package com.hedera.solo.operator.library.builder;

import io.fabric8.kubernetes.api.builder.Builder;
import io.fabric8.kubernetes.api.builder.Editable;
import io.fabric8.kubernetes.api.builder.Nested;

public class BuilderWrapper<T, B extends Builder<T>, E extends Editable<B>> {
    private final B builder;

    private BuilderWrapper(B builder) {
        this.builder = builder;
    }

    public static <T, B extends Builder<T>, E extends Editable<B>> BuilderWrapper<T, B, E> edit(E editable) {
        return new BuilderWrapper<>(editable.edit());
    }

    public BuilderWrapper<T, B, E> with(BuilderFunction<B> function) {
        for (Object object = function.apply(builder); object instanceof Nested<?>; object = ((Nested<?>) object).and()) {}
        return this;
    }

    public T build() {
        return builder.build();
    }
}
