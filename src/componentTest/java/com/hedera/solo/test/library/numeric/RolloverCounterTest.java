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

package com.hedera.solo.test.library.numeric;

import static org.assertj.core.api.Assertions.assertThat;

import com.hedera.solo.operator.library.numeric.RolloverCounter;
import org.junit.jupiter.api.Test;

public class RolloverCounterTest {
    void compare(final RolloverCounter counter, final long value) {
        assertThat(counter.compare(counter.rollover(value - 1), counter.rollover(value)))
                .isLessThan(0);
        assertThat(counter.compare(counter.rollover(value), counter.rollover(value)))
                .isEqualTo(0);
        assertThat(counter.compare(counter.rollover(value + 1), counter.rollover(value)))
                .isGreaterThan(0);
    }

    @Test
    public void test() {
        for (int bits = 2; bits <= 64; bits++) {
            RolloverCounter counter = new RolloverCounter(bits);
            for (long value = -8; value < 8; value++) {
                compare(counter, value - counter.half);
                compare(counter, value);
                compare(counter, value + counter.half);
            }
        }
    }
}
