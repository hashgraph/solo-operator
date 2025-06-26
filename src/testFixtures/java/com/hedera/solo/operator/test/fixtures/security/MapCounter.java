// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.fixtures.security;

import java.util.Collection;
import java.util.Map;
import java.util.function.Supplier;

/**
 * The MapCounter class is a generic utility that wraps a map to count and manage objects of type V, associated with
 * keys of type K.
 *
 * @param <K> The type of keys in the map.
 * @param <V> The type of values in the map, which are counted.
 */
public class MapCounter<K, V> {
    private final Map<K, ObjectCounter<V>> map;

    /**
     * Constructs a new MapCounter instance using the provided Supplier to create the internal map.
     *
     * @param supplier A Supplier that provides an instance of the map implementation.
     */
    public MapCounter(final Supplier<Map<K, ObjectCounter<V>>> supplier) {
        this.map = supplier.get();
    }

    /**
     * Decrements the counter associated with the given key and removes the entry if its counter reaches zero.
     *
     * @param key The key of the map entry to decrement and potentially remove.
     * @return The value associated with the key before decrementing, or null if the key does not exist or its counter
     *     is already zero.
     */
    public V decrementAndRemoveIfNone(final K key) {
        ObjectCounter<V> entry = map.get(key);
        ObjectCounter<V> removed = (entry != null && entry.counter.decrementAndGet() == 0) ? map.remove(key) : null;
        return removed != null ? removed.value : null;
    }

    /**
     * Checks if the map is empty.
     *
     * @return true if the map contains no key-value pairs, false otherwise
     */
    public boolean isEmpty() {
        return map.isEmpty();
    }

    /**
     * Puts a new value into the map with the specified key if it does not already exist, and increments its counter.
     *
     * @param key The key to be associated with the given value.
     * @param value The value to put into the map if the key does not exist.
     */
    public void putIfAbsentAndIncrement(final K key, final V value) {
        map.computeIfAbsent(key, k -> new ObjectCounter<>(value)).counter.incrementAndGet();
    }

    /**
     * Retrieves a collection of all values in the map.
     *
     * @return A Collection containing all the values from the map's object counters
     */
    public Collection<V> values() {
        return map.values().stream().map(ObjectCounter::value).toList();
    }
}
