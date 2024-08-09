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

package com.hedera.solo.operator.test.capsule;

import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.HAProxyCapsule;
import com.hedera.solo.operator.model.v1alpha1.HAProxyCapsuleBuilder;
import com.hedera.solo.operator.model.v1alpha1.HAProxyCapsuleStatus;
import com.hedera.solo.operator.test.HederaSdkClientFactory;
import io.fabric8.kubernetes.api.model.ObjectMeta;
import io.fabric8.kubernetes.client.CustomResource;
import io.quarkus.test.junit.QuarkusTest;
import java.lang.invoke.MethodHandles;
import java.util.List;
import java.util.Objects;
import java.util.Set;
import java.util.concurrent.TimeUnit;
import java.util.stream.Collectors;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.Test;

@QuarkusTest
public class HAProxyCapsuleTest extends ConsensusCapsuleTestAbstract {
    public HAProxyCapsuleTest() {
        setNamespace(MethodHandles.lookup().lookupClass().getSimpleName().toLowerCase());
    }

    @Override
    @AfterEach
    void cleanup() {
        kubernetesClient.resources(HAProxyCapsule.class).inNamespace(namespace).delete();
        super.cleanup();
    }

    @Override
    @Test
    void testSingleNodeDeployment() {
        super.testSingleNodeDeployment();
        createHAProxyCapsule();
    }

    @Override
    @Test
    void testMultiNodeDeployment() throws Exception {
        super.testMultiNodeDeployment();
        createHAProxyCapsule();
    }

    void createHAProxyCapsule() {
        String[] consensusNodeNames =
                kubernetesClient.resources(ConsensusCapsule.class).inNamespace(namespace).list().getItems().stream()
                        .map(CustomResource::getMetadata)
                        .map(ObjectMeta::getName)
                        .toArray(String[]::new);
        for (String name : consensusNodeNames) {
            HAProxyCapsule haProxyCapsule = new HAProxyCapsuleBuilder()
                    .editMetadata()
                    .withNamespace(namespace)
                    .withName(name)
                    .endMetadata()
                    .editSpec()
                    .editConsensus()
                    .withNode(name)
                    .endConsensus()
                    .endSpec()
                    .build();
            kubernetesClient.resource(haProxyCapsule).create();
        }
        await().atMost(30, TimeUnit.SECONDS).untilAsserted(() -> {
            List<HAProxyCapsule> proxies = kubernetesClient
                    .resources(HAProxyCapsule.class)
                    .inNamespace(namespace)
                    .list()
                    .getItems();
            assertThat(proxies).hasSize(consensusNodeNames.length);
            assertThat(proxies.stream()
                            .map(CustomResource::getMetadata)
                            .map(ObjectMeta::getName)
                            .collect(Collectors.toSet()))
                    .isEqualTo(Set.of(consensusNodeNames));
            assertThat(proxies.stream()).map(CustomResource::getStatus).allMatch(Objects::nonNull);
            assertThat(proxies.stream())
                    .map(CustomResource::getStatus)
                    .map(HAProxyCapsuleStatus::getPhase)
                    .allMatch(HAProxyCapsuleStatus.Phase.Running::equals);
        });
        await().atMost(90, TimeUnit.SECONDS).untilAsserted(() -> {
            HederaSdkClientFactory sdkClientFactory = new HederaSdkClientFactory(kubernetesClient, namespace);
            for (String nodeName : consensusNodeNames) {
                checkAddressBook(sdkClientFactory.haproxyContextCreator(nodeName), consensusNodeNames.length);
            }
        });
    }
}
