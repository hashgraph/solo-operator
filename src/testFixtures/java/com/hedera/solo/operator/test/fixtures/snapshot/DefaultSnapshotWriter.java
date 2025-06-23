// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.fixtures.snapshot;

import static com.hedera.solo.operator.test.fixtures.snapshot.CallStack.calledFrom;

import io.fabric8.kubernetes.client.utils.KubernetesSerialization;
import java.io.IOException;
import java.lang.reflect.Method;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Arrays;
import java.util.Objects;

/**
 * The {@code DefaultSnapshotWriter} class is responsible for writing snapshots of Kubernetes resources to disk. It
 * provides methods to generate and write snapshot files based on the provided objects.
 */
public class DefaultSnapshotWriter implements SnapshotWriter {
    private static final String SNAPSHOTS = "build/snapshots";
    private final KubernetesSerialization serialization = new KubernetesSerialization();
    private final Path snapshots;

    /** Constructs a {@code DefaultSnapshotWriter} object for the caller. */
    public DefaultSnapshotWriter() {
        this(calledFrom());
    }

    /**
     * Constructs a {@code DefaultSnapshotWriter} object using the calling method's stack frame.
     *
     * @param frame The stack frame containing the method information.
     */
    public DefaultSnapshotWriter(final StackWalker.StackFrame frame) {
        this(frame.getDeclaringClass().getCanonicalName() + "#" + frame.getMethodName());
    }

    /**
     * Constructs a {@code DefaultSnapshotWriter} object using the calling method.
     *
     * @param method The method information.
     */
    public DefaultSnapshotWriter(final Method method) {
        this(method.getDeclaringClass().getCanonicalName() + "#" + method.getName());
    }

    /**
     * Constructs a {@code DefaultSnapshotWriter} object with the specified name.
     *
     * @param name The unique identifier for the snapshot.
     */
    private DefaultSnapshotWriter(final String name) {
        snapshots = distinct(Path.of(SNAPSHOTS, name));
    }

    @Override
    public Path path(final String... source) {
        return path("", source);
    }

    @Override
    public void write(final Object object, final String... source) {
        String output;
        String suffix;
        if (object instanceof String string) {
            output = string;
            suffix = ".txt";
        } else {
            output = serialization.asYaml(object);
            suffix = ".yml";
        }
        Path path = path(suffix, source);
        path.getParent().toFile().mkdirs();
        try {
            Files.writeString(path, output);
        } catch (IOException ignored) {
            // If the file cannot be written, we simply ignore the exception.
        }
    }

    /**
     * Generates a distinct snapshot file path based on the provided base path and suffix.
     *
     * @param suffix The file name suffix.
     * @param source Additional strings used to generate the file path.
     * @return The generated path to the snapshot file
     */
    private Path path(final String suffix, final String... source) {
        return distinct(append(
                snapshots.resolve(Path.of(
                        "", Arrays.stream(source).filter(Objects::nonNull).toArray(String[]::new))),
                suffix));
    }

    /**
     * Appends a suffix to the provided path.
     *
     * @param path The base path.
     * @param suffix The file name suffix to be appended.
     * @return The new path with the appended suffix
     */
    private Path append(final Path path, final String suffix) {
        return Path.of(path + suffix);
    }

    /**
     * Ensures that a snapshot file does not already exist by adding a unique count if it does.
     *
     * @param path The base path to be checked and possibly modified.
     * @return The provided path or a new path with an incremented count suffix if the original path exists
     */
    private Path distinct(final Path path) {
        if (!path.toFile().exists()) {
            return path;
        }
        int count = Objects.requireNonNull(path.getParent()
                        .toFile()
                        .list((d, f) -> f.startsWith(path.getFileName().toString())))
                .length;
        return append(path, "." + count);
    }
}
