// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.generic;

import java.util.LinkedHashMap;
import java.util.Map;

/**
 * A Least Recently Used (LRU) cache implementation using a LinkedHashMap. The LRUCache evicts the least recently used
 * entries when its size exceeds the specified capacity.
 *
 * @param <K> the type of keys maintained by this map
 * @param <V> the type of mapped values
 */
public class LRUCache<K, V> extends LinkedHashMap<K, V> {
    private final int capacity;

    /**
     * Constructs an empty {@code LRUCache} instance with the specified capacity.
     *
     * @param capacity the maximum number of entries that will be stored in this cache
     */
    public LRUCache(int capacity) {
        super(capacity, 0.75f, true);
        this.capacity = capacity;
    }

    @Override
    protected boolean removeEldestEntry(Map.Entry<K, V> eldest) {
        return size() > capacity;
    }
}
