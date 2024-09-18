/*
 * Copyright (C) 2024 Hedera Hashgraph, LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package com.hedera.solo.test.framework;

import static com.hedera.solo.test.framework.KubernetesDevServiceExecutionListener.initialize;

import io.fabric8.kubernetes.client.KubernetesClient;
import io.javaoperatorsdk.operator.junit.AbstractOperatorExtension;
import io.javaoperatorsdk.operator.junit.LocallyRunOperatorExtension;
import org.junit.jupiter.api.extension.RegisterExtension;

public abstract class AbstractOperatorTest {
    @RegisterExtension
    private AbstractOperatorExtension operator = initialize(this::configure);

    public LocallyRunOperatorExtension.Builder builder(KubernetesClient client) {
        return LocallyRunOperatorExtension.builder().withKubernetesClient(client);
    }

    public AbstractOperatorExtension operator() {
        return operator;
    }

    protected abstract LocallyRunOperatorExtension.Builder configure(final KubernetesClient client);
}
