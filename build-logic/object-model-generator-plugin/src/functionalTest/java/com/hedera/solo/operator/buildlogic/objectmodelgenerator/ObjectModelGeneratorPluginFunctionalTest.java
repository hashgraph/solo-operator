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

package com.hedera.solo.operator.buildlogic.objectmodelgenerator;

import java.io.File;
import java.io.IOException;
import java.io.Writer;
import java.io.FileWriter;

import org.gradle.testkit.runner.GradleRunner;
import org.gradle.testkit.runner.BuildResult;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

import static org.junit.jupiter.api.Assertions.assertEquals;

/**
 * A simple functional test for the 'com.hedera.solo.operator.buildlogic.objectModelGenerator' plugin.
 */
class ObjectModelGeneratorPluginFunctionalTest {
    @TempDir
    File projectDir;

    @TempDir
    File sourceCrd;

    @TempDir
    File targetCrd;

    @TempDir
    File targetJava;

    private File getBuildFile() {
        return new File(projectDir, "build.gradle.kts");
    }

    private File getSettingsFile() {
        return new File(projectDir, "settings.gradle");
    }

    @Test
    void canRunTask() throws IOException {
        writeString(getSettingsFile(), "");
        writeString(getBuildFile(), """
                        plugins {
                          id("java")
                          id("com.hedera.solo.operator.buildlogic.objectModelGenerator")
                        }
                        objectModelGenerator {
                            main.sourceCrd = project.layout.projectDirectory.asFile.resolve("%s")
                            main.targetCrd = project.layout.buildDirectory.get().asFile.resolve("%s")
                            main.targetJava = project.layout.buildDirectory.get().asFile.resolve("%s")
                        }
                        """.formatted(sourceCrd, targetCrd, targetJava));

        // Run the build
        GradleRunner runner = GradleRunner.create();
        runner.forwardOutput();
        runner.withPluginClasspath();
        runner.withArguments("objectModelGenerator");
        runner.withProjectDir(projectDir);
        BuildResult result = runner.build();

        // Verify the result
        assertEquals(1, result.getTasks().size());
    }

    private void writeString(File file, String string) throws IOException {
        try (Writer writer = new FileWriter(file)) {
            writer.write(string);
        }
    }
}
