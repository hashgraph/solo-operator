// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.fixtures.snapshot;

import static com.hedera.solo.operator.test.fixtures.snapshot.DefaultSnapshot.snapshot;

import io.fabric8.kubernetes.client.KubernetesClient;
import org.eclipse.microprofile.config.Config;
import org.eclipse.microprofile.config.ConfigProvider;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.extension.ExtensionContext;
import org.junit.jupiter.api.extension.ParameterContext;
import org.junit.jupiter.api.extension.ParameterResolutionException;
import org.junit.jupiter.api.extension.ParameterResolver;

/**
 * The SnapshotFailureContextParameterResolver class is a JUnit parameter resolver that injects a SnapshotFailureContext
 * object into test methods annotated with @AfterEach.
 */
public class SnapshotFailureContextParameterResolver implements ParameterResolver {
    @Override
    public boolean supportsParameter(final ParameterContext parameterContext, final ExtensionContext extensionContext)
            throws ParameterResolutionException {
        return parameterContext.getParameter().getDeclaringExecutable().isAnnotationPresent(AfterEach.class)
                && parameterContext.getParameter().getType().isAssignableFrom(SnapshotFailureContext.class);
    }

    @Override
    public Object resolveParameter(final ParameterContext parameterContext, final ExtensionContext extensionContext)
            throws ParameterResolutionException {
        if (parameterContext.getParameter().getType().isAssignableFrom(SnapshotFailureContext.class)) {
            return new SnapshotFailureContext() {
                @Override
                public Snapshot client(final KubernetesClient client) {
                    return extensionContext.getExecutionException().isPresent() && isSnapshotEnabled()
                            ? snapshot(extensionContext.getRequiredTestMethod(), client)
                            : new NoOpSnapshot(client);
                }
            };
        }
        return null;
    }

    /**
     * Checks if the snapshot feature is enabled by reading the configuration value for "solo.test.snapshot.enabled".
     *
     * @return True if the snapshot feature is enabled, false otherwise
     */
    boolean isSnapshotEnabled() {
        Config config = ConfigProvider.getConfig();
        return "true".equals(config.getConfigValue("solo.test.snapshot.enabled").getValue());
    }
}
