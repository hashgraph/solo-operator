// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.generic;

import java.util.Arrays;
import java.util.List;
import java.util.stream.Stream;
import java.util.stream.StreamSupport;

public final class Iterables {
    private Iterables() {}

    @SafeVarargs
    public static <T> Stream<T> asStream(final Iterable<T>... iterables) {
        return Arrays.stream(iterables).flatMap(iter -> StreamSupport.stream(iter.spliterator(), false));
    }

    @SafeVarargs
    public static <T> List<T> asList(final Iterable<T>... iterables) {
        return asStream(iterables).toList();
    }
}
