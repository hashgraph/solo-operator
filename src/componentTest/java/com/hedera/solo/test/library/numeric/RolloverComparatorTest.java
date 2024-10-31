// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.test.library.numeric;

import static org.assertj.core.api.Assertions.assertThat;

import com.hedera.solo.operator.library.numeric.RolloverComparator;
import org.junit.jupiter.api.Test;

public class RolloverComparatorTest {
    void compare(final RolloverComparator cmp, final long value) {
        assertThat(cmp.compare(value - 1, value)).isLessThan(0);
        assertThat(cmp.compare(value, value)).isEqualTo(0);
        assertThat(cmp.compare(value + 1, value)).isGreaterThan(0);
    }

    @Test
    public void test() {
        for (int bits = 2; bits <= 64; bits++) {
            RolloverComparator cmp = new RolloverComparator(bits);
            for (long value = -8; value < 8; value++) {
                compare(cmp, value - cmp.half);
                compare(cmp, value);
                compare(cmp, value + cmp.half);
            }
        }
    }
}
