// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.builder;

import io.fabric8.kubernetes.api.builder.Builder;

/**
 * Functional interface representing a function that operates on a Kubernetes builder.
 *
 * @param <B> The type of the builder, extending Builder<?>
 */
@FunctionalInterface
public interface BuilderFunction<B extends Builder<?>> {
    /**
     * Applies the function to the given builder and returns the result.
     *
     * @param builder The builder instance
     * @return The result of applying the function to the builder
     * @throws Exception if an error occurs during the application of the function
     */
    Object applyChecked(final B builder) throws Exception;

    /**
     * Applies the function to the given builder and handles any checked exceptions by wrapping them in a
     * RuntimeException.
     *
     * @param builder The builder instance
     * @return The result of applying the function to the builder, or null if the function returns void
     */
    default Object apply(final B builder) {
        try {
            return applyChecked(builder);
        } catch (Exception e) {
            throw e instanceof RuntimeException r ? r : new RuntimeException(e);
        }
    }
}
