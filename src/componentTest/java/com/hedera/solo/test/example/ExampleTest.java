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

import static com.hedera.solo.operator.test.fixtures.snapshot.Snapshot.snapshot;
import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

import com.hedera.solo.operator.test.fixtures.annotations.AfterTest;
import com.hedera.solo.operator.test.v1alpha1.Example;
import com.hedera.solo.operator.test.v1alpha1.ExampleBuilder;
import com.hedera.solo.test.framework.AbstractOperatorTest;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.Pod;
import io.fabric8.kubernetes.api.model.PodBuilder;
import io.javaoperatorsdk.operator.junit.LocallyRunOperatorExtension;
import java.lang.reflect.Method;
import java.time.Duration;
import java.util.Map;
import org.junit.jupiter.api.Test;

public class ExampleTest extends AbstractOperatorTest {
    @Override
    protected LocallyRunOperatorExtension.Builder configure(LocallyRunOperatorExtension.Builder builder) {
        return builder.withReconciler(new ExampleReconciler());
    }

    @AfterTest
    void afterTest(Method method, Throwable throwable) throws Exception {
        if (throwable != null) {
            snapshot(method, operator().getKubernetesClient(), operator().getNamespace())
                    .exec(
                            "create-files",
                            "app=test",
                            "root",
                            """
                            mkdir -p /tmp/a/b/c
                            echo aaa > /tmp/a/aaa
                            echo bbb > /tmp/a/b/bbb
                            """)
                    .exec("list-files", "app=test", "root", "ls -al /tmp");
            // operator().getKubernetesClient().pods().inNamespace(operator().getNamespace()).withName("demo-pod1").file("/tmp/a/").copy(Path.of("/tmp/"))
        }
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

        for (int i = 0; i < 2; ++i) {
            String name = "demo-pod" + i;
            Pod aPod = new PodBuilder()
                    .withNewMetadata()
                    .withName(name)
                    .withLabels(Map.of("app", "test"))
                    .endMetadata()
                    .withNewSpec()
                    //                .withShareProcessNamespace()
                    .addNewContainer()
                    .withName("root")
                    .withImage("alpine:latest")
                    .withCommand(
                            "/bin/sh",
                            "-c",
                            """
                            echo he}{o >> /proc/1/fd/1;
                            echo goodbye >> /proc/1/fd/1;
                            tail -f /dev/null
                            """)
                    .endContainer()
                    .endSpec()
                    .build();
            operator().create(aPod);
            await().atMost(Duration.ofSeconds(30)).untilAsserted(() -> {
                assertThat(operator().get(Pod.class, name)).isNotNull();
            });
        }

        //        fail();

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
