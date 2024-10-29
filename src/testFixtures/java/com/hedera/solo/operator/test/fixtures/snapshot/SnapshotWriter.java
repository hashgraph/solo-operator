// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.fixtures.snapshot;

import java.nio.file.Path;

public interface SnapshotWriter {
    Path path(final String... source);

    void write(final Object object, final String... source);
}
