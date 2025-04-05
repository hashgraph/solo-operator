// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.test.framework;

import static com.hedera.solo.test.framework.KubernetesDevServiceExecutionListener.initialize;

import io.fabric8.kubernetes.client.KubernetesClient;
import io.javaoperatorsdk.operator.junit.AbstractOperatorExtension;
import io.javaoperatorsdk.operator.junit.LocallyRunOperatorExtension;
import org.junit.jupiter.api.extension.RegisterExtension;

public abstract class AbstractOperatorTest {
    @RegisterExtension
    private AbstractOperatorExtension operator = initialize(this::configure);

    public LocallyRunOperatorExtension.Builder builder(final KubernetesClient client) {
        return LocallyRunOperatorExtension.builder().withKubernetesClient(client);
    }

    public AbstractOperatorExtension operator() {
        return operator;
    }

    protected abstract LocallyRunOperatorExtension.Builder configure(final KubernetesClient client);
}
