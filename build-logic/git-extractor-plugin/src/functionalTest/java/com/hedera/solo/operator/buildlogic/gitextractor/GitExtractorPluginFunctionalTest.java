// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.buildlogic.gitextractor;

import static org.junit.jupiter.api.Assertions.*;

import java.io.File;
import java.io.FileWriter;
import java.io.IOException;
import java.io.Writer;
import org.gradle.testkit.runner.BuildResult;
import org.gradle.testkit.runner.GradleRunner;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

/** A simple functional test for the 'com.hedera.solo.operator.buildlogic.gitExtractor' plugin. */
class GitExtractorPluginFunctionalTest {
    @TempDir
    File projectDir;

    private File getBuildFile() {
        return new File(projectDir, "build.gradle.kts");
    }

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
                        """);

        // Run the build
        GradleRunner runner = GradleRunner.create();
        runner.forwardOutput();
        runner.withPluginClasspath();
        runner.withArguments("gitExtractor");
        runner.withProjectDir(projectDir);
        BuildResult result = runner.build();

        // Verify the result
        assertEquals(1, result.getTasks().size());
    }

    private void writeString(final File file, final String string) throws IOException {
        try (Writer writer = new FileWriter(file)) {
            writer.write(string);
        }
    }
}
