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

/**
 * The TestOperatorManager class is responsible for managing the lifecycle of the operator during test execution. It
 * checks various conditions to determine if it should start or stop the operator based on the testing environment.
 */
@ApplicationScoped
public class TestOperatorManager {
    Config config = ConfigProvider.getConfig();

    @Inject
    protected Operator operator;

    /**
     * This method is called when the application starts and initializes the operator if it should be managed.
     *
     * @param ev The StartupEvent triggered at startup time.
     */
    void onStart(@Observes StartupEvent ev) {
        if (manageOperator()) {
            operator.start();
        }
    }

    /**
     * This method is called when the application shuts down and stops the operator if it should be managed.
     *
     * @param event The ShutdownEvent triggered at shutdown time.
     */
    void onShutdown(@Observes ShutdownEvent event) {
        if (manageOperator()) {
            operator.stop();
        }
    }

    /**
     * Determines whether the operator should be managed based on various test conditions.
     *
     * @return true if the operator should be managed according to the defined conditions, otherwise false
     */
    boolean manageOperator() {
        return (isTestUsingGradle() || isTestUsingIntelliJ()) && !isTestUsingExternalOperatorInstance();
    }

    /**
     * Checks if the test is being run using Gradle.
     *
     * @return true if the test is running with Gradle, otherwise false
     */
    boolean isTestUsingGradle() {
        return null != config.getConfigValue("org.gradle.test.worker").getValue();
    }

    /**
     * Checks if the test is being run using IntelliJ IDEA which supports JUnit5.
     *
     * @return true if the test is running with IntelliJ IDEA, otherwise false
     */
    boolean isTestUsingIntelliJ() {
        return null != ClassLoader.getSystemClassLoader().getDefinedPackage("com.intellij.junit5");
    }

    /**
     * Checks if an external operator instance is being used for the test.
     *
     * @return true if an external operator instance is being used, otherwise false
     */
    boolean isTestUsingExternalOperatorInstance() {
        return null
                != config.getConfigValue("solo.operator.test-using-external-operator-instance")
                        .getValue();
    }
}
