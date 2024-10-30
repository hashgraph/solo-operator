// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.fixtures.snapshot;

public enum NonNamespaced {
    INCLUDE(true),
    EXCLUDE(false);

    final boolean include;

    NonNamespaced(final boolean include) {
        this.include = include;
    }
}
