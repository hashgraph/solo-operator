// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.fixtures;

import org.eclipse.microprofile.config.Config;
import org.eclipse.microprofile.config.ConfigProvider;

public class Environment {
    public static boolean isTestUsingGradleOrIntelliJ() {
        return isTestUsingGradle() || isTestUsingIntelliJ();
    }

    public static boolean isTestUsingGradle() {
        Config config = ConfigProvider.getConfig();
        return null != config.getConfigValue("org.gradle.test.worker").getValue();
    }

    public static boolean isTestUsingIntelliJ() {
        return null != ClassLoader.getSystemClassLoader().getDefinedPackage("com.intellij.junit5");
    }

    public static boolean isSnapshotEnabled() {
        Config config = ConfigProvider.getConfig();
        return "true".equals(config.getConfigValue("solo.test.snapshot.enabled").getValue());
    }
}
