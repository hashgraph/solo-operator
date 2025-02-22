// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.capsule;

import static com.hedera.solo.operator.reconciler.common.Constants.LABEL_K8S_MANAGED_BY;
import static com.hedera.solo.operator.reconciler.common.Constants.SOLO_OPERATOR;
import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsuleStatus;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationBuilder;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationSpec;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.reconciler.networkoperation.NetworkOperationSequenceReconciler;
import io.fabric8.kubernetes.api.model.Pod;
import io.quarkus.test.junit.QuarkusTest;
import java.util.List;
import java.util.Map;
import java.util.concurrent.TimeUnit;
import org.junit.jupiter.api.Test;

@QuarkusTest
public class NetworkOperationSequenceReconcilerTest extends ConsensusCapsuleTestAbstract {

    @Override
    @Test
    void testSingleNodeDeployment() throws Exception {
        super.testSingleNodeDeployment();
    }

    @Override
    @Test
    void testMultiNodeDeployment() throws Exception {
        super.testMultiNodeDeployment();
    }

    @Override
    protected void createNetworkOperation(List<ConsensusCapsule> consensusCapsules) throws InterruptedException {
        for (ConsensusCapsule consensusCapsule : consensusCapsules) {
            String name = consensusCapsule.getMetadata().getName();
            ConsensusCapsule capsule =
                    kubernetesClient.resource(consensusCapsule).get();
            assertThat(capsule).isNotNull();
        }

        final NetworkOperation networkGenesis = new NetworkOperationBuilder()
                .withNewMetadata()
                .withName("network-genesis")
                .withNamespace(namespace)
                .withLabels(Map.of(LABEL_K8S_MANAGED_BY, SOLO_OPERATOR))
                .endMetadata()
                .withNewSpec()
                .withOrbit(orbit)
                .withVersion(NetworkOperationSequenceReconciler.VERSION)
                .withMode(NetworkOperationSpec.Mode.NetworkGenesis)
                .endSpec()
                .withNewStatus()
                .withPhase(NetworkOperationStatus.Phase.Pending)
                .endStatus()
                .build();
        kubernetesClient.resource(networkGenesis).create();

        Thread.sleep(1000); // add delay after NetworkGenesis CR creation

        // create few more NetworkCheck CRs with some delay to test that these runs after network-genesis
        // Note: The name of the NetworkCheck CR should be unique and in chronological order as these are created with
        // the same timestamp, so that it doesn't conflict with the NetworkGenesis CR
        final int totalNetworkChecks = 3;
        for (int i = 0; i < totalNetworkChecks; i++) {
            final NetworkOperation networkCheck = new NetworkOperationBuilder()
                    .withNewMetadata()
                    .withName("network-check-%02d".formatted(i)) // padded with 0 so that it can be ordered by name
                    .withNamespace(namespace)
                    .withLabels(Map.of(LABEL_K8S_MANAGED_BY, SOLO_OPERATOR))
                    .endMetadata()
                    .withNewSpec()
                    .withOrbit(orbit)
                    .withVersion(NetworkOperationSequenceReconciler.VERSION)
                    .withMode(NetworkOperationSpec.Mode.NetworkCheck)
                    .endSpec()
                    .withNewStatus()
                    .withPhase(NetworkOperationStatus.Phase.Pending)
                    .endStatus()
                    .build();
            kubernetesClient.resource(networkCheck).create();
        }

        await().atMost(600, TimeUnit.SECONDS).untilAsserted(() -> {
            NetworkOperation netInit = kubernetesClient
                    .resources(NetworkOperation.class)
                    .inNamespace(namespace)
                    .withName("network-genesis")
                    .get();
            assertThat(netInit).isNotNull();
            assertThat(netInit.getStatus()).isNotNull();
            assertThat(netInit.getStatus().getPhase()).isEqualTo(NetworkOperationStatus.Phase.Succeeded);
        });

        await().atMost(300, TimeUnit.SECONDS).untilAsserted(() -> {
            for (int i = 0; i < totalNetworkChecks; i++) {
                NetworkOperation netCheck = kubernetesClient
                        .resources(NetworkOperation.class)
                        .inNamespace(namespace)
                        .withName("network-check-%02d".formatted(i)) // padded with 0 so that it can be ordered by name
                        .get();
                assertThat(netCheck).isNotNull();
                assertThat(netCheck.getStatus()).isNotNull();
                assertThat(netCheck.getStatus().getPhase()).isEqualTo(NetworkOperationStatus.Phase.Succeeded);
            }
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
