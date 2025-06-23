// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.buildlogic.crdgenerator;

import static org.junit.jupiter.api.Assertions.*;

import java.io.File;
import java.io.FileWriter;
import java.io.IOException;
import java.io.Writer;
import org.gradle.testkit.runner.BuildResult;
import org.gradle.testkit.runner.GradleRunner;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

/** A simple functional test for the 'com.hedera.solo.operator.buildlogic.crdGenerator' plugin. */
class CrdGeneratorPluginFunctionalTest {
    @TempDir
    File projectDir;

    private File getBuildFile() {
        return new File(projectDir, "build.gradle.kts");
    }

    private File getSettingsFile() {
        return new File(projectDir, "settings.gradle.kts");
    }

    private File getJavaSourceFile() {
        return new File(projectDir, "src/main/java/com/example/Example.java");
    }

    @Test
    void canRunTask() throws IOException {
        writeString(getSettingsFile(), "");
        writeString(
                getBuildFile(),
                """
                        plugins {
                          id("java")
                          id("com.hedera.solo.operator.buildlogic.crdGenerator")
                        }
                        crdGenerator {
                          register(sourceSets.main) {}
                        }
                        """);
        getJavaSourceFile().getParentFile().mkdirs();
        writeString(getJavaSourceFile(), """
                public class Example {}
                """);

        // Run the build
        GradleRunner runner = GradleRunner.create();
        runner.forwardOutput();
        runner.withPluginClasspath();
        runner.withArguments("compileJava", "generateCrd");
        runner.withProjectDir(projectDir);
        BuildResult result = runner.build();

        // Verify the result
        assertEquals(2, result.getTasks().size());
    }

    private void writeString(final File file, final String string) throws IOException {
        try (Writer writer = new FileWriter(file)) {
            writer.write(string);
        }
    }
}
