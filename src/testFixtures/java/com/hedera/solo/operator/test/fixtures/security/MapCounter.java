// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.fixtures.security;

import java.util.Collection;
import java.util.Map;
import java.util.function.Supplier;

public class MapCounter<K, V> {
    private final Map<K, ObjectCounter<V>> map;

    public MapCounter(final Supplier<Map<K, ObjectCounter<V>>> supplier) {
        this.map = supplier.get();
    }

    public V decrementAndRemoveIfNone(final K key) {
        ObjectCounter<V> entry = map.get(key);
        ObjectCounter<V> removed = (entry != null && entry.counter.decrementAndGet() == 0) ? map.remove(key) : null;
        return removed != null ? removed.value : null;
    }

    public boolean isEmpty() {
        return map.isEmpty();
    }

    public void putIfAbsentAndIncrement(final K key, final V value) {
        map.computeIfAbsent(key, k -> new ObjectCounter<>(value)).counter.incrementAndGet();
    }

    public Collection<V> values() {
        return map.values().stream().map(ObjectCounter::value).toList();
    }
}
