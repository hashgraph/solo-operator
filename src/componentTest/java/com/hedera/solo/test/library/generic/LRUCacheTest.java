// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.test.library.generic;

import static org.assertj.core.api.Assertions.assertThat;

import com.hedera.solo.operator.library.generic.LRUCache;
import java.util.Set;
import org.junit.jupiter.api.Test;

public class LRUCacheTest {
    @Test
    void test() {
        LRUCache<Integer, String> cache = new LRUCache<>(3);
        assertThat(cache.size()).isEqualTo(0);
        cache.put(1, "one");
        assertThat(cache.size()).isEqualTo(1);
        assertThat(Set.of(1)).allMatch(cache::containsKey);
        cache.put(2, "two");
        assertThat(cache.size()).isEqualTo(2);
        assertThat(Set.of(1, 2)).allMatch(cache::containsKey);
        cache.put(3, "three");
        assertThat(cache.size()).isEqualTo(3);
        assertThat(Set.of(1, 2, 3)).allMatch(cache::containsKey);
        cache.put(4, "four");
        assertThat(cache.size()).isEqualTo(3);
        assertThat(Set.of(2, 3, 4)).allMatch(cache::containsKey);

        // LRU (not FIFO)
        cache.get(4);
        cache.get(3);
        cache.get(2);
        cache.put(1, "one");
        assertThat(cache.size()).isEqualTo(3);
        assertThat(Set.of(1, 2, 3)).allMatch(cache::containsKey);
    }
}
