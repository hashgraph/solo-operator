// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.test.framework;

import static com.hedera.solo.test.framework.KubernetesDevServiceExecutionListener.operatorExtensionBuilder;

import io.javaoperatorsdk.operator.junit.AbstractOperatorExtension;
import io.javaoperatorsdk.operator.junit.LocallyRunOperatorExtension;
import org.junit.jupiter.api.extension.RegisterExtension;

public abstract class AbstractOperatorTest {
    @RegisterExtension
    private AbstractOperatorExtension operator =
            configure(operatorExtensionBuilder()).build();

    public AbstractOperatorExtension operator() {
        return operator;
    }

    protected abstract LocallyRunOperatorExtension.Builder configure(LocallyRunOperatorExtension.Builder builder);
}
