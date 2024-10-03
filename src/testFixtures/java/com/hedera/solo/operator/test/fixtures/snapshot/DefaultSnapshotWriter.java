/*
 * Copyright (C) 2024 Hedera Hashgraph, LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package com.hedera.solo.operator.test.fixtures.snapshot;

import static com.hedera.solo.operator.test.fixtures.snapshot.CallStack.calledFrom;

import io.fabric8.kubernetes.client.utils.KubernetesSerialization;
import java.io.IOException;
import java.lang.reflect.Method;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Arrays;
import java.util.Objects;

public class DefaultSnapshotWriter implements SnapshotWriter {
    private static final String SNAPSHOTS = "build/snapshots";
    private final KubernetesSerialization serialization = new KubernetesSerialization();
    private final Path snapshots;

    public DefaultSnapshotWriter() {
        this(calledFrom());
    }

    public DefaultSnapshotWriter(final StackWalker.StackFrame frame) {
        this(frame.getDeclaringClass().getCanonicalName() + "#" + frame.getMethodName());
    }

    public DefaultSnapshotWriter(final Method method) {
        this(method.getDeclaringClass().getCanonicalName() + "#" + method.getName());
    }

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
        }
    }

    private Path path(final String suffix, final String... source) {
        return distinct(append(
                snapshots.resolve(Path.of(
                        "", Arrays.stream(source).filter(Objects::nonNull).toArray(String[]::new))),
                suffix));
    }

    private Path append(final Path path, final String suffix) {
        return Path.of(path + suffix);
    }

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
