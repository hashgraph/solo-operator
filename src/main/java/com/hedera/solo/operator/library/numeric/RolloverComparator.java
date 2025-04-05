// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.numeric;

import java.util.Comparator;

/**
 * Compare relative order of numbers in rollover sequence assuming they differ by less than half the length of the
 * sequence. For example with 8 bits, 5 value sequence may be ordered as: ... 253, 254, 255, 0, 1, 2 ...
 */
public class RolloverComparator implements Comparator<Long> {
    public final long half;
    public final long mask;

    public RolloverComparator(final int bits) {
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
