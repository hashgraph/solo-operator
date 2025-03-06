// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.capsule;

import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsuleStatus;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.test.utils.TemplateLoader;
import io.fabric8.kubernetes.api.model.Pod;
import io.quarkus.test.junit.QuarkusTest;
import java.util.List;
import java.util.concurrent.TimeUnit;
import org.junit.jupiter.api.Test;

@QuarkusTest
public class NetworkUpgradeOperationTest extends ConsensusCapsuleTestAbstract {

    @Override
    @Test
    void testSingleNodeDeployment() throws Exception {
        super.testSingleNodeDeployment();
        createNetworkUpgradeOperation(consensusCapsules);
    }

    @Override
    @Test
    void testMultiNodeDeployment() throws Exception {
        super.testMultiNodeDeployment();
        createNetworkUpgradeOperation(consensusCapsules);
    }

    protected void createNetworkUpgradeOperation(List<ConsensusCapsule> consensusCapsules) throws InterruptedException {
        for (ConsensusCapsule consensusCapsule : consensusCapsules) {
            String name = consensusCapsule.getMetadata().getName();
            ConsensusCapsule capsule =
                    kubernetesClient.resource(consensusCapsule).get();
            assertThat(capsule).isNotNull();
        }

        final NetworkOperation networkUpgrade =
                TemplateLoader.load(NetworkOperation.class, TemplateLoader.TEMPLATE_NETWORK_UPGRADE);
        networkUpgrade.getMetadata().setNamespace(orbit);
        kubernetesClient.resource(networkUpgrade).create();

        await().atMost(600, TimeUnit.SECONDS).untilAsserted(() -> {
            NetworkOperation netInit = kubernetesClient
                    .resources(NetworkOperation.class)
                    .inNamespace(namespace)
                    .withName("network-upgrade")
                    .get();
            assertThat(netInit).isNotNull();
            assertThat(netInit.getStatus()).isNotNull();
            assertThat(netInit.getStatus().getPhase()).isEqualTo(NetworkOperationStatus.Phase.Pending);
            assertThat(netInit.getStatus().getSoftwareSourceStatus()).isNotNull();
            assertThat(netInit.getStatus().getSoftwareSourceStatus().getConsensusNode())
                    .isNotNull();
        });

        for (ConsensusCapsule consensusCapsule : consensusCapsules) {
            String name = consensusCapsule.getMetadata().getName();
            ConsensusCapsule capsule =
                    kubernetesClient.resource(consensusCapsule).get();
            assertThat(capsule).isNotNull();

            Pod pod = kubernetesClient
                    .resources(Pod.class)
                    .inNamespace(namespace)
                    .withName(name + "-pod")
                    .get();
            assertThat(pod).isNotNull();
            assertThat(pod.getStatus()).isNotNull();
            assertThat(pod.getStatus().getPhase()).isEqualTo("Running");

            assertThat(capsule.getStatus().getPhase()).isEqualTo(ConsensusCapsuleStatus.Phase.Running);
        }
    }
}
