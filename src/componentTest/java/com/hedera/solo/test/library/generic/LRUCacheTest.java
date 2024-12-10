// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.test.library.generic;

import static org.assertj.core.api.Assertions.assertThat;

import com.hedera.solo.operator.library.generic.LRUCache;
import org.junit.jupiter.api.Test;

public class LRUCacheTest {
    @Test
    void test() {
        LRUCache<Integer, String> cache = new LRUCache<>(3);
        cache.put(1, "one");
        cache.put(2, "two");
        cache.put(3, "three");
        cache.put(4, "four");
        assertThat(cache.containsKey(1)).isFalse();
        assertThat(cache.get(2)).isEqualTo("two");
        assertThat(cache.get(3)).isEqualTo("three");
        assertThat(cache.get(4)).isEqualTo("four");
    }
}
