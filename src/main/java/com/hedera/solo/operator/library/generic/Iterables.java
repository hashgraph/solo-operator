/*
 * Copyright (C) 2024 Hedera Hashgraph, LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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
