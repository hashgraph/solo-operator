// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.generic;

import java.util.Collections;
import java.util.List;
import java.util.Map;
import java.util.function.Supplier;

/** The {@code Nullable} class provides utility methods to handle nullable values in a functional and safe manner. */
public class Nullable {
    /** Private constructor to prevent instantiation of this utility class. */
    private Nullable() {}

    /**
     * Returns the result of invoking the supplier if it is non-null, otherwise returns the alternate value.
     *
     * @param <T> The type of the object returned by the supplier.
     * @param supplier A supplier whose result may be null.
     * @param alternate The value to return if the supplier's result is null.
     * @return The result of invoking the supplier, or the alternate value if the supplier's result is null
     */
    public static <T> T orElse(final Supplier<T> supplier, final T alternate) {
        return orElseGet(supplier, () -> alternate);
    }

    /**
     * Returns the result of invoking the supplier if it is non-null, otherwise returns the result of invoking the
     * alternate supplier.
     *
     * @param <T> The type of the object returned by the supplier and alternate supplier.
     * @param supplier A supplier whose result may be null.
     * @param alternate A supplier whose result may be used as an alternative if the first supplier's result is null.
     * @return The result of invoking the supplier, or the result of invoking the alternate supplier if the supplier's
     *     result is null.
     */
    public static <T> T orElseGet(final Supplier<T> supplier, final Supplier<T> alternate) {
        try {
            T value = supplier.get();
            if (value != null) {
                return value;
            }
        } catch (NullPointerException ignored) {
            // If the supplier throws a NullPointerException, we treat it as if it returned null.
            // This is useful for suppliers that may throw exceptions when they are not supposed to.
        }
        return alternate.get();
    }

    /**
     * Returns an empty list if the supplier does not provide a non-null result, otherwise returns the provided list.
     *
     * @param <T> The type of elements in the list returned by the supplier.
     * @param supplier A supplier whose result may be null or a list.
     * @return An empty list if the supplier's result is null, otherwise the result of invoking the supplier
     */
    public static <T> List<T> orEmptyList(final Supplier<List<T>> supplier) {
        return orElseGet(supplier, Collections::emptyList);
    }

    /**
     * Returns an empty map if the supplier does not provide a non-null result, otherwise returns the provided map.
     *
     * @param <K> The type of keys in the map returned by the supplier.
     * @param <V> The type of values in the map returned by the supplier.
     * @param supplier A supplier whose result may be null or a map.
     * @return An empty map if the supplier's result is null, otherwise the result of invoking the supplier
     */
    public static <K, V> Map<K, V> orEmptyMap(final Supplier<Map<K, V>> supplier) {
        return orElseGet(supplier, Collections::emptyMap);
    }

    /**
     * Returns null if the supplier does not provide a non-null result, otherwise returns the provided value.
     *
     * @param <T> The type of the object returned by the supplier.
     * @param supplier A supplier whose result may be null.
     * @return Null if the supplier's result is null, otherwise the result of invoking the supplier
     */
    public static <T> T orNull(final Supplier<T> supplier) {
        return orElse(supplier, null);
    }
}
