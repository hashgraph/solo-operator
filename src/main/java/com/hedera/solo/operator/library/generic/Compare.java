package com.hedera.solo.operator.library.generic;

import java.util.Objects;
import java.util.function.Function;

public class Compare {
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
