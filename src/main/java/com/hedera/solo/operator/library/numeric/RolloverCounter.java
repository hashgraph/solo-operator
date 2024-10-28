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

package com.hedera.solo.operator.library.numeric;

import java.util.Comparator;

public class RolloverCounter implements Comparator<Long> {
    public final long half;
    public final long mask;

    public RolloverCounter(final int bits) {
        half = 1L << bits - 1;
        mask = half | half - 1;
    }

    public long rollover(final long value) {
        return value & mask;
    }

    @Override
    public int compare(final Long a, final Long b) {
        long diff = a - b & mask;
        return (diff | half) == half ? 0 : Long.compareUnsigned(half, diff);
    }
}
