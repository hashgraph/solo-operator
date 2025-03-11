// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.fixtures.security;

import java.util.concurrent.atomic.AtomicLong;

public class ObjectCounter<T> {
    public final AtomicLong counter = new AtomicLong();
    public final T value;

    public ObjectCounter(final T value) {
        this.value = value;
    }

    public T value() {
        return value;
    }
}
