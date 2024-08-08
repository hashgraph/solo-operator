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

package com.hedera.solo.operator.test;

import com.dajudge.kindcontainer.KindContainer;
import com.hedera.solo.operator.test.framework.manytoone.StatusUpdaterReconciler;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.fabric8.kubernetes.client.KubernetesClientBuilder;
import io.javaoperatorsdk.operator.junit.LocallyRunOperatorExtension;
import java.lang.invoke.MethodHandles;
import org.junit.jupiter.api.Disabled;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.RegisterExtension;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.testcontainers.junit.jupiter.Testcontainers;

@Disabled
@Testcontainers
public class NestedDemo {
    private final Logger log = LoggerFactory.getLogger(MethodHandles.lookup().lookupClass());

    KindContainer<?> kind;

    KubernetesClient get() {
        kind = new KindContainer<>();
        kind.start();
        return new KubernetesClientBuilder().withConfig(kind.getKubeconfig()).build();
    }

    @RegisterExtension
    LocallyRunOperatorExtension operator = LocallyRunOperatorExtension.builder()
            .withReconciler(new StatusUpdaterReconciler())
            .withKubernetesClient(get())
            .build();

    @Test
    void test() {
        log.info("namespace: {}", operator.getNamespace());
    }
}
