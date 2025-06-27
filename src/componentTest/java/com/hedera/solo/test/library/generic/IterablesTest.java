// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.test.library.generic;

import static org.junit.jupiter.api.Assertions.assertEquals;

import com.hedera.solo.operator.library.generic.Iterables;
import java.util.List;
import org.junit.jupiter.api.Test;

/**
 * The Iterables class contains utility methods for working with collections, particularly converting between different
 * types of lists.
 */
public class IterablesTest {
    @Test
    public void test() {
        List<Integer> list = Iterables.asList(List.of(1, 2, 3), List.of(4, 5, 6));
        assertEquals(List.of(1, 2, 3, 4, 5, 6), list);
    }
}
