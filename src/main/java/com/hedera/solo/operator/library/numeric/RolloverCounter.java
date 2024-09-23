package com.hedera.solo.operator.library.numeric;

import java.util.Comparator;

public class RolloverCounter implements Comparator<Long> {
    public final long half;
    public final long mask;

    public RolloverCounter(int bits) {
        half = 1L << bits - 1;
        mask = half | half - 1;
    }

    public long rollover(long value) {
        return value & mask;
    }

    @Override
    public int compare(Long a, Long b) {
        long diff = a - b & mask;
        return (diff | half) == half ? 0 : Long.compareUnsigned(half, diff);
    }
}
