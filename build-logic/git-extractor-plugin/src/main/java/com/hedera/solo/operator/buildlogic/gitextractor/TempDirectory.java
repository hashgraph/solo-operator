// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.buildlogic.gitextractor;

import java.io.File;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.attribute.FileAttribute;
import java.util.Comparator;
import java.util.stream.Stream;

/**
 * The {@code TempDirectory} class provides a temporary directory that can be created and automatically deleted when no longer in use.
 */
public class TempDirectory implements AutoCloseable {
    /**
     * The path to the temporary directory.
     */
    public final Path path;

    /**
     * Constructs a new {@code TempDirectory} with the specified prefix and file attributes.
     *
     * @param prefix the prefix for the generated temporary directory name
     * @param attrs  the file attributes to apply to the created directory
     * @throws IOException if an I/O error occurs during creation of the temporary directory
     */
    public TempDirectory(final String prefix, final FileAttribute<?>... attrs) throws IOException {
        path = Files.createTempDirectory(prefix, attrs);
    }

    @Override
    public void close() throws IOException {
        try (Stream<Path> walk = Files.walk(path)) {
            walk.sorted(Comparator.reverseOrder()).map(Path::toFile).forEach(File::delete);
        }
    }
}
