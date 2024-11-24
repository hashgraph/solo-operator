// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.capsule;

import static com.hedera.solo.operator.reconciler.common.Constants.LABEL_SELECTOR_CONSENSUS_NODE_APP;
import static com.hedera.solo.operator.reconciler.common.Constants.LABEL_SELECTOR_HAPROXY_APP;
import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

import com.hedera.solo.operator.model.v1alpha1.HAProxyCapsule;
import com.hedera.solo.operator.model.v1alpha1.HAProxyCapsuleBuilder;
import com.hedera.solo.operator.model.v1alpha1.HAProxyCapsuleSpec;
import com.hedera.solo.operator.model.v1alpha1.HAProxyCapsuleStatus;
import com.hedera.solo.operator.reconciler.common.Metadata;
import com.hedera.solo.operator.test.fixtures.hederaclient.HederaSdkClientFactory;
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
import org.junit.jupiter.api.Disabled;
import org.junit.jupiter.api.Test;

@Disabled
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
        Long[] consensusNodeIds = consensusNodeIds();
        for (Long nodeId : consensusNodeIds) {
            HAProxyCapsule haProxyCapsule = new HAProxyCapsuleBuilder()
                    .editMetadata()
                    .withNamespace(namespace)
                    .withName(Metadata.prepareNodeName(nodeId))
                    .endMetadata()
                    .editSpec()
                    .withOrbit(orbit)
                    .withNodeId(nodeId)
                    .endSpec()
                    .build();
            kubernetesClient.resource(haProxyCapsule).create();
        }
        await().atMost(60, TimeUnit.SECONDS).untilAsserted(() -> {
            List<HAProxyCapsule> proxies = kubernetesClient
                    .resources(HAProxyCapsule.class)
                    .inNamespace(namespace)
                    .list()
                    .getItems();
            assertThat(proxies).hasSize(consensusNodeIds.length);
            assertThat(proxies.stream()
                            .map(CustomResource::getSpec)
                            .map(HAProxyCapsuleSpec::getNodeId)
                            .collect(Collectors.toSet()))
                    .isEqualTo(Set.of(consensusNodeIds));
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
        Supplier<Map<String, Long>> currentGenerations = () -> kubernetesClient
                .resources(Deployment.class)
                .inNamespace(namespace)
                .withLabel(LABEL_SELECTOR_HAPROXY_APP)
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
                .withLabel(LABEL_SELECTOR_CONSENSUS_NODE_APP)
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
                .withLabel(LABEL_SELECTOR_CONSENSUS_NODE_APP)
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
