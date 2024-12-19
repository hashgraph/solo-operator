// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.buildlogic.gitextractor;

import java.io.File;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.attribute.FileAttribute;
import java.util.Comparator;
import java.util.stream.Stream;

public class TempDirectory implements AutoCloseable {
    public final Path path;

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
