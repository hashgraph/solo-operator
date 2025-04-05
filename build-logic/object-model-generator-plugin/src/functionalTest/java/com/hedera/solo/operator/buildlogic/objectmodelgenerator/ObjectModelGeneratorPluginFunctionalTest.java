// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.buildlogic.objectmodelgenerator;

import static org.junit.jupiter.api.Assertions.assertEquals;

import java.io.File;
import java.io.FileWriter;
import java.io.IOException;
import java.io.Writer;
import org.gradle.testkit.runner.BuildResult;
import org.gradle.testkit.runner.GradleRunner;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

/**
 * A simple functional test for the 'com.hedera.solo.operator.buildlogic.objectModelGenerator' plugin.
 */
class ObjectModelGeneratorPluginFunctionalTest {
    @TempDir
    File projectDir;

    @TempDir
    File sourceCrd;

    @TempDir
    File sourceOpenAPI;

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
                            register(sourceSets.main) {
                                sourceCrd = file("%s")
                                sourceOpenAPI = file("%s")
                            }
                        }
                        """.formatted(sourceCrd, sourceOpenAPI));

        // Run the build
        GradleRunner runner = GradleRunner.create();
        runner.forwardOutput();
        runner.withPluginClasspath();
        runner.withArguments("generateObjectModel");
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
