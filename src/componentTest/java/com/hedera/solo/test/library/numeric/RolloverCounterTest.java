package com.hedera.solo.test.library.numeric;

import static org.assertj.core.api.Assertions.assertThat;

import com.hedera.solo.operator.library.numeric.RolloverCounter;
import java.util.List;
import org.junit.jupiter.api.Test;

public class RolloverCounterTest {
    void compare(RolloverCounter counter, long value) {
        assertThat(counter.compare(counter.rollover(value - 1), counter.rollover(value))).isLessThan(0);
        assertThat(counter.compare(counter.rollover(value), counter.rollover(value))).isEqualTo(0);
        assertThat(counter.compare(counter.rollover(value + 1), counter.rollover(value))).isGreaterThan(0);
    }

    @Test
    public void test() {
        for (int bits : List.of(2, 3, 4, 62, 63, 64)) {
            RolloverCounter counter = new RolloverCounter(bits);
            assertThat(counter.half).isEqualTo(1L << bits - 1);
            for (long value = -8; value < 8; value++) {
                compare(counter, value - counter.half);
                compare(counter, value);
                compare(counter, value + counter.half);
            }
        }
    }
}
