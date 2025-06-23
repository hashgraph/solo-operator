// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.fixtures.snapshot;

/** Enumeration representing whether a resource should be included or excluded from namespaced operations. */
public enum NonNamespaced {
    INCLUDE(true),
    EXCLUDE(false);

    final boolean include;

    /**
     * Constructs a NonNamespaced enum instance with the specified inclusion status.
     *
     * @param include true if the resource should be included in namespaced operations, false otherwise.
     */
    NonNamespaced(final boolean include) {
        this.include = include;
    }
}
