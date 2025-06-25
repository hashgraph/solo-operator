// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.generic;

import java.util.Arrays;
import java.util.List;
import java.util.stream.Stream;
import java.util.stream.StreamSupport;

/**
 * The {@code Iterables} class provides utility methods to convert an array of {@link Iterable}s into a single
 * {@link Stream} or {@link List}.
 */
public final class Iterables {
    /** Private constructor to prevent instantiation of this utility class. */
    private Iterables() {}

    /**
     * Converts an array of {@link Iterable}s into a {@link Stream}.
     *
     * @param iterables An array of {@code Iterable<T>} objects.
     * @param <T> The type of elements in the iterable.
     * @return A single {@link Stream} containing all elements from the input iterables
     */
    @SafeVarargs
    public static <T> Stream<T> asStream(final Iterable<T>... iterables) {
        return Arrays.stream(iterables).flatMap(iter -> StreamSupport.stream(iter.spliterator(), false));
    }

    /**
     * Converts an array of {@link Iterable}s into a {@link List}.
     *
     * @param iterables An array of {@code Iterable<T>} objects.
     * @param <T> The type of elements in the iterable.
     * @return A {@link List} containing all elements from the input iterables
     */
    @SafeVarargs
    public static <T> List<T> asList(final Iterable<T>... iterables) {
        return asStream(iterables).toList();
    }
}
