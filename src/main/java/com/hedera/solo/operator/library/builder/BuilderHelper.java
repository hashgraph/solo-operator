package com.hedera.solo.operator.library.builder;

import io.fabric8.kubernetes.api.builder.Builder;
import io.fabric8.kubernetes.api.builder.Editable;
import io.fabric8.kubernetes.api.builder.Nested;
import java.util.function.Function;

public class BuilderHelper<T, B extends Builder<T>, E extends Editable<B>> {
    private final B builder;

    private BuilderHelper(B builder) {
        this.builder = builder;
    }

    public static <T, B extends Builder<T>, E extends Editable<B>> BuilderHelper<T, B, E> edit(E editable) {
        return new BuilderHelper<>(editable.edit());
    }

    public BuilderHelper<T, B, E> with(Function<B, ?> function) {
        for (Object object = function.apply(builder); object instanceof Nested<?>; object = ((Nested<?>) object).and()) {}
        return this;
    }

    public T build() {
        return builder.build();
    }
}
