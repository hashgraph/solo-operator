// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.fixtures.snapshot;

import java.nio.file.Path;

/** The SnapshotWriter interface provides methods to create a path and write an object to the snapshot. */
public interface SnapshotWriter {
    /**
     * Generates a path based on the provided sources.
     *
     * @param source The sources used to generate the path.
     * @return A Path object representing the generated path
     */
    Path path(final String... source);

    /**
     * Writes an object to the snapshot using the specified sources as part of the path.
     *
     * @param object The object to be written to the snapshot.
     * @param source The sources used to generate the path where the object will be written.
     */
    void write(final Object object, final String... source);
}
