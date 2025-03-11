// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.generic;

import java.util.Collections;
import java.util.List;
import java.util.Map;
import java.util.function.Supplier;

public class Nullable {
    private Nullable() {}

    public static <T> T orElse(final Supplier<T> supplier, final T alternate) {
        return orElseGet(supplier, () -> alternate);
    }

    public static <T> T orElseGet(final Supplier<T> supplier, final Supplier<T> alternate) {
        try {
            T value = supplier.get();
            return value == null ? alternate.get() : value;
        } catch (NullPointerException e) {
            return alternate.get();
        }
    }

    public static <T> List<T> orEmptyList(final Supplier<List<T>> supplier) {
        return orElseGet(supplier, Collections::emptyList);
    }

    public static <K, V> Map<K, V> orEmptyMap(final Supplier<Map<K, V>> supplier) {
        return orElseGet(supplier, Collections::emptyMap);
    }

    public static <T> T orNull(final Supplier<T> supplier) {
        return orElse(supplier, null);
    }
}
