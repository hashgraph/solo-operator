// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.numeric;

import java.util.Comparator;

/**
 * The RolloverComparator class provides a comparator for numeric values that supports rollover sequences, such as those
 * used in cyclic counters or circular buffers. It assumes the numbers differ by less than half the length of the
 * sequence.
 */
public class RolloverComparator implements Comparator<Long> {
    public final long half;
    public final long mask;

    /**
     * Constructs a new RolloverComparator with the specified number of bits, defining the range and handling
     * overflow/underflow scenarios.
     *
     * @param bits The number of bits used to define the sequence length.
     */
    public RolloverComparator(final int bits) {
        half = 1L << bits - 1;
        mask = half | half - 1;
    }

    /**
     * Computes the rollover value for a given number, ensuring it fits within the defined range by applying the mask.
     *
     * @param value The numeric value to be adjusted for potential overflow.
     * @return The value adjusted to fit within the bit size specified during construction
     */
    public long rollover(final long value) {
        return value & mask;
    }

    @Override
    public int compare(final Long a, final Long b) {
        long diff = a - b & mask;
        return (diff | half) == half ? 0 : Long.compareUnsigned(half, diff);
    }
}
