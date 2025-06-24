// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.generic;

import java.util.Objects;
import java.util.function.Function;

/** Utility class providing methods for comparing objects based on specific properties using functions. */
public final class Compare {
    /** Private constructor to prevent instantiation of this utility class. */
    private Compare() {}

    /**
     * Checks if all specified property mappings of two objects are equal.
     *
     * @param <T> the type of the objects being compared
     * @param a the first object to compare
     * @param b the second object to compare
     * @param mappers an array of functions that map the objects to properties for comparison
     * @return true if all property mappings are equal, false otherwise
     */
    @SafeVarargs
    public static <T> boolean allMatch(final T a, final T b, final Function<T, ?>... mappers) {
        for (Function<T, ?> mapper : mappers) {
            if (!Objects.equals(mapper.apply(a), mapper.apply(b))) {
                return false;
            }
        }
        return true;
    }
}
