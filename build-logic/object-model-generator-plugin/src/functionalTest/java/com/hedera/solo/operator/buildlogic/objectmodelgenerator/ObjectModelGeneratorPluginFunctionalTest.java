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

    /**
     * Retrieves the build file for the Gradle project.
     *
     * @return The build file as a {@link File} object
     */
    private File getBuildFile() {
        return new File(projectDir, "build.gradle.kts");
    }

    /**
     * Retrieves the settings file for the Gradle project.
     *
     * @return The settings file as a {@link File} object
     */
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

    /**
     * Writes a string to a file.
     *
     * @param file The file to which the string will be written.
     * @param string The string to write to the file.
     * @throws IOException If there is an issue with file handling or I/O operations.
     */
    private void writeString(File file, String string) throws IOException {
        try (Writer writer = new FileWriter(file)) {
            writer.write(string);
        }
    }
}
