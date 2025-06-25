// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.builder;

import io.fabric8.kubernetes.api.builder.Builder;
import io.fabric8.kubernetes.api.builder.Editable;
import io.fabric8.kubernetes.api.builder.Nested;

/**
 * The `BuilderWrapper` class provides a wrapper around Kubernetes builders to facilitate method chaining and simplify
 * the construction of Kubernetes resources. It is designed to be used with any builder that implements the
 * {@link Builder}, {@link Editable}, and {@link Nested} interfaces from the Fabric8 Kubernetes Client library.
 *
 * @param <T> The type of the resource being built.
 * @param <B> The specific type of the builder implementation.
 * @param <E> The specific type of the editable implementation.
 */
public class BuilderWrapper<T, B extends Builder<T>, E extends Editable<B>> {
    private final B builder;

    /**
     * Private constructor to initialize the wrapper with a given builder.
     *
     * @param builder The builder instance to be wrapped.
     */
    private BuilderWrapper(final B builder) {
        this.builder = builder;
    }

    /**
     * A factory method that returns a new `BuilderWrapper` instance initialized with the editable resource's builder.
     *
     * @param <T> The type of the resource being built.
     * @param <B> The specific type of the builder implementation.
     * @param <E> The specific type of the editable implementation.
     * @param editable The editable resource to be wrapped and modified.
     * @return A new `BuilderWrapper` instance
     */
    public static <T, B extends Builder<T>, E extends Editable<B>> BuilderWrapper<T, B, E> edit(final E editable) {
        return new BuilderWrapper<>(editable.edit());
    }

    /**
     * Applies a function to the builder and modifies it according to the logic defined in the function.
     *
     * @param function A lambda expression defining how the builder should be modified.
     * @return The current `BuilderWrapper` instance for method chaining
     */
    public BuilderWrapper<T, B, E> with(final BuilderFunction<B> function) {
        for (Object object = function.apply(builder);
                object instanceof Nested<?>;
                object = ((Nested<?>) object).and()) {
            // Continue applying the function until we reach the end of the nested structure
        }
        return this;
    }

    /**
     * Builds the resource using the wrapped builder and returns it.
     *
     * @return The built resource instance
     */
    public T build() {
        return builder.build();
    }
}
