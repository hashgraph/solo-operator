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

package com.hedera.solo.test.example;

import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

import com.hedera.solo.operator.test.fixtures.annotations.SoloTest;
import com.hedera.solo.operator.test.fixtures.snapshot.SnapshotFailureContext;
import com.hedera.solo.operator.test.v1alpha1.Example;
import com.hedera.solo.operator.test.v1alpha1.ExampleBuilder;
import com.hedera.solo.test.framework.AbstractOperatorTest;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.javaoperatorsdk.operator.junit.LocallyRunOperatorExtension;
import java.time.Duration;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.Test;

@SoloTest
public class ExampleTest extends AbstractOperatorTest {
    @Override
    protected LocallyRunOperatorExtension.Builder configure(final KubernetesClient client) {
        return builder(client).withReconciler(new ExampleReconciler());
    }

    @AfterEach
    void afterEach(final SnapshotFailureContext snapshot) {
        snapshot.client(operator().getKubernetesClient())
                .namespaces(operator().getNamespace())
                .namespacedResources();
    }

    @Test
    void test() {
        // create custom resource
        Example example = new ExampleBuilder()
                .editMetadata()
                .withName("example")
                .endMetadata()
                .editSpec()
                .withMessage("hello world!")
                .endSpec()
                .build();
        example = operator().create(example);

        // verify dependent resource was is created as expected
        await().atMost(Duration.ofSeconds(30)).untilAsserted(() -> {
            ConfigMap configMap = operator().get(ConfigMap.class, "example");
            assertThat(configMap).hasFieldOrPropertyWithValue("metadata.name", "example");
            assertThat(configMap).hasFieldOrPropertyWithValue("data.message", "hello world!");
        });

        // update custom resource
        example = example.edit()
                .editSpec()
                .withMessage("goodbye world!")
                .endSpec()
                .build();
        example = operator().replace(example);

        // verify update is reflected in dependent resource as expected
        await().atMost(Duration.ofSeconds(30)).untilAsserted(() -> {
            ConfigMap configMap = operator().get(ConfigMap.class, "example");
            assertThat(configMap).hasFieldOrPropertyWithValue("metadata.name", "example");
            assertThat(configMap).hasFieldOrPropertyWithValue("data.message", "goodbye world!");
        });

        // delete custom resource
        operator().delete(example);

        // verify dependent resource has been deleted
        await().atMost(Duration.ofSeconds(30)).untilAsserted(() -> {
            ConfigMap configMap = operator().get(ConfigMap.class, "example");
            assertThat(configMap).isNull();
        });
    }
}
