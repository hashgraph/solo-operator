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

import static com.hedera.solo.operator.reconciler.capsule.consensus.ConsensusCapsuleConstants.LABEL_SELECTOR_CONSENSUS_NODE;
import static com.hedera.solo.operator.reconciler.capsule.haproxy.HAProxyCapsuleConstants.LABEL_SELECTOR_HAPROXY;
import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

import com.hedera.solo.operator.model.v1alpha1.HAProxyCapsule;
import com.hedera.solo.operator.model.v1alpha1.HAProxyCapsuleBuilder;
import com.hedera.solo.operator.model.v1alpha1.HAProxyCapsuleStatus;
import com.hedera.solo.operator.test.framework.HederaSdkClientFactory;
import io.fabric8.kubernetes.api.model.ObjectMeta;
import io.fabric8.kubernetes.api.model.Secret;
import io.fabric8.kubernetes.api.model.apps.Deployment;
import io.fabric8.kubernetes.client.CustomResource;
import io.quarkus.test.junit.QuarkusTest;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Set;
import java.util.concurrent.TimeUnit;
import java.util.function.Supplier;
import java.util.stream.Collectors;
import org.junit.jupiter.api.Test;

@QuarkusTest
public class HAProxyCapsuleTest extends ConsensusCapsuleTestAbstract {
    @Override
    void cleanup() {
        kubernetesClient.resources(HAProxyCapsule.class).inNamespace(namespace).delete();
        super.cleanup();
    }

    @Override
    @Test
    void testSingleNodeDeployment() throws Exception {
        super.testSingleNodeDeployment();
        createHAProxyCapsule();
        simulateCertificateUpdate();
    }

    @Override
    @Test
    void testMultiNodeDeployment() throws Exception {
        super.testMultiNodeDeployment();
        createHAProxyCapsule();
        simulateCertificateUpdate();
    }

    void createHAProxyCapsule() throws Exception {
        String[] consensusNodeNames = consensusNodeNames();
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

        // test with client
        checkAddressBook(HederaSdkClientFactory::haproxyContextCreator);
    }

    void simulateCertificateUpdate() throws Exception {
        Supplier<Map<String, Long>> currentGenerations =
                () -> kubernetesClient
                        .resources(Deployment.class)
                        .inNamespace(namespace)
                        .withLabel(LABEL_SELECTOR_HAPROXY)
                        .list()
                        .getItems()
                        .stream()
                        .collect(Collectors.toMap(
                                deployment -> deployment.getMetadata().getName(),
                                deployment -> deployment.getStatus().getObservedGeneration()));

        // initial observed generations
        Map<String, Long> initialGenerations = currentGenerations.get();

        // modify hedera.crt in consensus secret
        for (Secret secret : kubernetesClient
                .secrets()
                .inNamespace(namespace)
                .withLabel(LABEL_SELECTOR_CONSENSUS_NODE)
                .list()
                .getItems()) {
            String fieldName = secret.getData().keySet().stream()
                    .filter(s -> s.startsWith("hedera") && s.endsWith(".crt"))
                    .findAny()
                    .orElse("hedera.crt");
            kubernetesClient.resource(secret).edit(s -> s.edit()
                    .addToData(fieldName + ".orig", s.getData().get(fieldName))
                    .addToData(fieldName, "AA==")
                    .build());
        }

        // verify haproxy deployments are updated
        await().atMost(30, TimeUnit.SECONDS).untilAsserted(() -> {
            assertThat(currentGenerations.get().entrySet())
                    .noneMatch(e -> Objects.equals(initialGenerations.get(e.getKey()), e.getValue()));
        });

        // restore hedera.crt consensus secret
        for (Secret secret : kubernetesClient
                .secrets()
                .inNamespace(namespace)
                .withLabel(LABEL_SELECTOR_CONSENSUS_NODE)
                .list()
                .getItems()) {
            String fieldName = secret.getData().keySet().stream()
                    .filter(s -> s.startsWith("hedera") && s.endsWith(".crt.orig"))
                    .map(s -> s.substring(0, s.length() - 5))
                    .findAny()
                    .orElse("hedera.crt");
            kubernetesClient.resource(secret).edit(s -> s.edit()
                    .addToData(fieldName, s.getData().get(fieldName + ".orig"))
                    .removeFromData(fieldName + ".orig")
                    .build());
        }

        // test with client
        checkAddressBook(HederaSdkClientFactory::haproxyContextCreator);
    }
}
