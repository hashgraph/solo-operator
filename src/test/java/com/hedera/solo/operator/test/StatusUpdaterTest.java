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

import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

import com.hedera.solo.operator.model.v1alpha1.StatusUpdater;
import com.hedera.solo.operator.test.framework.TestFramework;
import com.hedera.solo.operator.test.framework.manytoone.StatusUpdaterReconciler;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.javaoperatorsdk.operator.junit.LocallyRunOperatorExtension;
import io.quarkus.test.junit.QuarkusTest;
import io.quarkus.test.junit.callback.QuarkusTestAfterAllCallback;
import io.quarkus.test.junit.callback.QuarkusTestAfterEachCallback;
import io.quarkus.test.junit.callback.QuarkusTestBeforeClassCallback;
import io.quarkus.test.junit.callback.QuarkusTestBeforeEachCallback;
import io.quarkus.test.junit.callback.QuarkusTestContext;
import io.quarkus.test.junit.callback.QuarkusTestMethodContext;
import jakarta.inject.Inject;
import java.lang.invoke.MethodHandles;
import java.time.Duration;
import java.util.ArrayList;
import java.util.List;
import org.junit.jupiter.api.Disabled;
import org.junit.jupiter.api.RepeatedTest;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.junit.jupiter.api.extension.RegisterExtension;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

@Disabled
@QuarkusTest
//@ExtendWith(LocallyRunOperatorExtension.class)
public class StatusUpdaterTest implements QuarkusTestBeforeClassCallback, QuarkusTestBeforeEachCallback, QuarkusTestAfterAllCallback, QuarkusTestAfterEachCallback {
    private final Logger log = LoggerFactory.getLogger(MethodHandles.lookup().lookupClass());

    @Inject
    @SuppressWarnings("CdiInjectionPointsInspection")
    protected KubernetesClient client;

//    @RegisterExtension
    LocallyRunOperatorExtension operator;
//        = LocallyRunOperatorExtension.builder()
//            .withReconciler(new StatusUpdaterReconciler())
//            .withKubernetesClient(client)
//            .oneNamespacePerClass(true)
//            .build();

    @Test
    void test() throws Exception {
        StatusUpdater statusUpdater = new StatusUpdater();
        List<StatusUpdater> list = new ArrayList<>();
        for (int i = 0; i < 100; i++) {
            statusUpdater.getMetadata().setName("example-" + i);
            list.add(operator.create(statusUpdater));
        }

        await().atMost(Duration.ofSeconds(90)).untilAsserted(() -> {
            ConfigMap cm = operator.get(ConfigMap.class, "status-updater");
            ConfigMap cm2 = operator.get(ConfigMap.class, "status-updater-no-ssa");
            assertThat(cm).isNotNull();
            assertThat(cm2).isNotNull();
            for (StatusUpdater updater : list) {
                assertThat(cm.hasOwnerReferenceFor(updater)).isTrue();
                assertThat(cm2.hasOwnerReferenceFor(updater)).isTrue();
            }
        });

        operator.delete(operator.get(ConfigMap.class, "status-updater"));
        operator.delete(operator.get(ConfigMap.class, "status-updater-no-ssa"));

        await().atMost(Duration.ofSeconds(90)).untilAsserted(() -> {
            ConfigMap cm = operator.get(ConfigMap.class, "status-updater");
            ConfigMap cm2 = operator.get(ConfigMap.class, "status-updater-no-ssa");
            assertThat(cm).isNotNull();
            assertThat(cm2).isNotNull();
            for (StatusUpdater updater : list) {
                assertThat(cm.hasOwnerReferenceFor(updater)).isTrue();
                assertThat(cm2.hasOwnerReferenceFor(updater)).isTrue();
            }
        });

        list.forEach(operator::delete);

        await().atMost(Duration.ofSeconds(30)).untilAsserted(() -> {
            assertThat(operator.get(ConfigMap.class, "status-updater")).isNull();
            assertThat(operator.get(ConfigMap.class, "status-updater-no-ssa")).isNull();
        });
    }

    @Override
    public void afterAll(QuarkusTestContext context) {

    }

    @Override
    public void afterEach(QuarkusTestMethodContext context) {

    }

    @Override
    public void beforeClass(Class<?> testClass) {

    }

    @Override
    public void beforeEach(QuarkusTestMethodContext context) {
        operator = LocallyRunOperatorExtension.builder()
                .withReconciler(new StatusUpdaterReconciler())
                .withKubernetesClient(client)
                .oneNamespacePerClass(true)
                .build();
//        operator.beforeEach();
    }
}
