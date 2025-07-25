// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.test.framework;

import static com.hedera.solo.test.framework.KubernetesDevServiceExecutionListener.initialize;

import io.fabric8.kubernetes.client.KubernetesClient;
import io.javaoperatorsdk.operator.junit.AbstractOperatorExtension;
import io.javaoperatorsdk.operator.junit.LocallyRunOperatorExtension;
import org.junit.jupiter.api.extension.RegisterExtension;

/**
 * Abstract base class for operator tests. This class provides a JUnit extension to run the operator locally and
 * includes methods to configure the builder with a Kubernetes client.
 */
public abstract class AbstractOperatorTest {
    @RegisterExtension
    private AbstractOperatorExtension operator = initialize(this::configure);

    /**
     * Returns a builder for configuring the operator extension with a Kubernetes client.
     *
     * @param client The Kubernetes client to be used by the operator extension.
     * @return A builder instance for LocallyRunOperatorExtension
     */
    public LocallyRunOperatorExtension.Builder builder(final KubernetesClient client) {
        return LocallyRunOperatorExtension.builder().withKubernetesClient(client);
    }

    /**
     * Returns the operator extension instance.
     *
     * @return The AbstractOperatorExtension instance
     */
    public AbstractOperatorExtension operator() {
        return operator;
    }

    /**
     * Abstract method to configure the LocallyRunOperatorExtension with a Kubernetes client. This method should be
     * implemented by subclasses to provide specific configuration for the operator extension.
     *
     * @param client The Kubernetes client to be used for configuring the operator extension.
     * @return A builder instance for LocallyRunOperatorExtension
     */
    protected abstract LocallyRunOperatorExtension.Builder configure(KubernetesClient client);
}
