// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.fixtures.security;

import java.util.concurrent.atomic.AtomicLong;

/**
 * A utility class to count objects of a generic type, maintaining both the count and the value itself.
 *
 * @param <T> The type of the counted object.
 */
public class ObjectCounter<T> {
    /** The counter that keeps track of the number of occurrences of the value. */
    public final AtomicLong counter = new AtomicLong();
    /** The value associated with this counter. */
    public final T value;

    /**
     * Constructs an ObjectCounter with a specific value.
     *
     * @param value The initial value to be stored in the counter.
     */
    public ObjectCounter(final T value) {
        this.value = value;
    }

    /**
     * Retrieves the value associated with this counter.
     *
     * @return The value of type T
     */
    public T value() {
        return value;
    }
}
