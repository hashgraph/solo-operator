// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.framework;

import io.javaoperatorsdk.operator.Operator;
import io.quarkus.runtime.ShutdownEvent;
import io.quarkus.runtime.StartupEvent;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.enterprise.event.Observes;
import jakarta.inject.Inject;
import org.eclipse.microprofile.config.Config;
import org.eclipse.microprofile.config.ConfigProvider;

@ApplicationScoped
public class TestOperatorManager {
    Config config = ConfigProvider.getConfig();

    @Inject
    protected Operator operator;

    void onStart(@Observes StartupEvent ev) {
        if (manageOperator()) {
            operator.start();
        }
    }

    void onShutdown(@Observes ShutdownEvent event) {
        if (manageOperator()) {
            operator.stop();
        }
    }

    boolean manageOperator() {
        return (isTestUsingGradle() || isTestUsingIntelliJ()) && !isTestUsingExternalOperatorInstance();
    }

    boolean isTestUsingGradle() {
        return null != config.getConfigValue("org.gradle.test.worker").getValue();
    }

    boolean isTestUsingIntelliJ() {
        return null != ClassLoader.getSystemClassLoader().getDefinedPackage("com.intellij.junit5");
    }

    boolean isTestUsingExternalOperatorInstance() {
        return null
                != config.getConfigValue("solo.operator.test-using-external-operator-instance")
                        .getValue();
    }
}
