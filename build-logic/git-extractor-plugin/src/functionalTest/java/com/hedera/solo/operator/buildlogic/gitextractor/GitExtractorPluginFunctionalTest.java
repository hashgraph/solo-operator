// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.buildlogic.gitextractor;

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
 * A simple functional test for the 'com.hedera.solo.operator.buildlogic.gitExtractor' plugin.
 */
class GitExtractorPluginFunctionalTest {
    @TempDir
    File projectDir;

    /**
     * Retrieves the build file for the project directory.
     *
     * @return The build file as a {@link File}
     */
    private File getBuildFile() {
        return new File(projectDir, "build.gradle.kts");
    }

    /**
     * Retrieves the settings file for the project directory.
     *
     * @return The settings file as a {@link File}
     */
    private File getSettingsFile() {
        return new File(projectDir, "settings.gradle.kts");
    }

    @Test
    void canRunTask() throws IOException {
        writeString(getSettingsFile(), "");
        writeString(
                getBuildFile(),
                """
                        plugins {
                          id("java")
                          id("com.hedera.solo.operator.buildlogic.gitExtractor")
                        }
                        gitExtractor {
                            register(sourceSets.main) {
                                addResources(
                                    "https://github.com/hashgraph/hedera-services",
                                    "refs/tags/v0.56.5",
                                    "hedera-node/configuration/mainnet",
                                    "import/hedera/mainnet"
                                )
                            }
                        }
                        """);

        // Run the build
        GradleRunner runner = GradleRunner.create();
        runner.forwardOutput();
        runner.withPluginClasspath();
        runner.withArguments("generateGitExtractor");
        runner.withProjectDir(projectDir);
        BuildResult result = runner.build();

        // Verify the result
        assertEquals(1, result.getTasks().size());
    }

    /**
     * Writes a string to a file.
     *
     * @param file The file to which the string will be written.
     * @param string The string content to write into the file.
     * @throws IOException If there is an issue with file I/O operations.
     */
    private void writeString(final File file, final String string) throws IOException {
        try (Writer writer = new FileWriter(file)) {
            writer.write(string);
        }
    }
}
