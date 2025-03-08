// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.capsule;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.operator.reconciler.common.Constants.LABEL_K8S_MANAGED_BY;
import static com.hedera.solo.operator.reconciler.common.Constants.LABEL_NODE_NAME;
import static com.hedera.solo.operator.reconciler.common.Constants.SOLO_OPERATOR;
import static com.hedera.solo.operator.reconciler.networkoperation.genesis.steps.CreateClusterConfigStep.computeNodeExternalAddress;
import static com.hedera.solo.operator.reconciler.networkoperation.genesis.steps.CreateClusterConfigStep.computeNodeInternalAddress;
import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

import com.hedera.solo.operator.model.commonspec.AddressBookEntry;
import com.hedera.solo.operator.model.commonspec.AddressBookEntryBuilder;
import com.hedera.solo.operator.model.commonspec.ImageRepository;
import com.hedera.solo.operator.model.commonspec.ImageVerificationSpec;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationSpec;
import com.hedera.solo.operator.model.v1alpha1.networkoperationspec.NetworkUpgrade;
import com.hedera.solo.operator.reconciler.common.Constants;
import com.hedera.solo.operator.reconciler.common.Metadata;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.tasks.NowFrozenTask;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.Pod;
import io.quarkus.test.junit.QuarkusTest;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.concurrent.TimeUnit;
import org.junit.jupiter.api.Test;

@QuarkusTest
public class NetworkUpgradeOperationTest extends ConsensusCapsuleTestAbstract {

    @Override
    @Test
    void testSingleNodeDeployment() throws Exception {
        super.testSingleNodeDeployment();
        triggerNowFrozen(consensusCapsules);
    }

    @Override
    @Test
    void testMultiNodeDeployment() throws Exception {
        super.testMultiNodeDeployment();
        triggerNowFrozen(consensusCapsules);
    }

    protected void triggerNowFrozen(List<ConsensusCapsule> consensusCapsules) throws InterruptedException {
        final List<AddressBookEntry> addressBookEntries = new ArrayList<>();
        for (ConsensusCapsule consensusCapsule : consensusCapsules) {
            ConsensusCapsule capsule =
                    kubernetesClient.resource(consensusCapsule).get();
            assertThat(capsule).isNotNull();

            // prepare address book entry
            String domain = "";
            String nodeName = Metadata.prepareNodeName(capsule);
            String namespace = capsule.getMetadata().getNamespace();
            String internal = computeNodeInternalAddress(domain, capsule, namespace);
            String external = computeNodeExternalAddress(capsule, nodeName, namespace);
            Long nodeId = capsule.getSpec().getNodeId();
            String accountId = capsule.getSpec().getAccountId();
            final AddressBookEntry abe = new AddressBookEntryBuilder()
                    .withNodeId(nodeId)
                    .withExternalHostname(external)
                    .withExternalPort(Constants.GOSSIP_PORT)
                    .withInternalHostname(internal)
                    .withInternalPort(Constants.GOSSIP_PORT)
                    .withNickName(nodeName)
                    .withSelfName(nodeName)
                    .withNodeAccount(accountId)
                    .withWeight(1L)
                    .build();
            addressBookEntries.add(abe);
        }

        NetworkOperation upgrade = edit(new NetworkOperation())
                .with(metadata -> metadata.editMetadata()
                        .withName("network-upgrade")
                        .withNamespace(orbit)
                        .withLabels(Map.of(LABEL_K8S_MANAGED_BY, SOLO_OPERATOR)))
                .with(spec -> spec.editSpec()
                        .withMode(NetworkOperationSpec.Mode.NetworkUpgrade)
                        .withOrbit(orbit)
                        .editNetworkUpgrade()
                        .withWorkflowStep(NetworkUpgrade.WorkflowStep.NowFrozen)
                        .editChangesets()
                        .editConsensusNodeChangeset()
                        .editSoftwareVersionSource()
                        .withImageRepositories(edit(new ImageRepository())
                                .with(repo -> repo.withRepository("swirldslabs.jfrog.io/consensus-node-docker-dev")
                                        .withImageName("consensus-node")
                                        .withImageTag("0.59.0-main.x02ba8df"))
                                .build())
                        .withImageVerificationSpec(edit(new ImageVerificationSpec())
                                .with(
                                        ivs -> ivs.withOs(ImageVerificationSpec.Os.linux)
                                                .withArchitecture(ImageVerificationSpec.Architecture.arm64)
                                                .withLayerHashes(
                                                        "sha256:cf3813c609184ac4f804a288f5bf39adebb84b1ace91c9a49445f5fa757b56bc",
                                                        "sha256:4f4fb700ef54461cfa02571ae0db9a0dc1e0cdb5577484a6d75e68dc38e8acc1"))
                                .build()))
                .with(ab -> ab.editSpec().editNetworkUpgrade().withAddressBook(addressBookEntries))
                .build();

        kubernetesClient.resource(upgrade).create();
        checkStatus(consensusCapsules);
    }

    private void checkStatus(List<ConsensusCapsule> consensusCapsules) {
        await().atMost(10, TimeUnit.MINUTES).untilAsserted(() -> {
            Thread.sleep(2000);
            NetworkOperation netOp = kubernetesClient
                    .resources(NetworkOperation.class)
                    .inNamespace(namespace)
                    .withName("network-upgrade")
                    .get();
            assertThat(netOp).isNotNull();
            assertThat(netOp.getStatus()).isNotNull();
            assertThat(NowFrozenTask.isDone(netOp)).isTrue();
            assertThat(netOp.getStatus().getSoftwareSourceStatus()).isNotNull();
            assertThat(netOp.getStatus().getSoftwareSourceStatus().getConsensusNode())
                    .isNotNull();
            assertThat(netOp.getStatus().getUpgradedNodes().size()).isEqualTo(consensusCapsules.size());

            ConfigMap cc = kubernetesClient
                    .resources(ConfigMap.class)
                    .inNamespace(namespace)
                    .withName(Constants.CLUSTER_CONFIG_NAME)
                    .get();
            assertThat(cc).isNotNull();
        });

        for (ConsensusCapsule consensusCapsule : consensusCapsules) {
            String name = consensusCapsule.getMetadata().getName();
            ConsensusCapsule capsule =
                    kubernetesClient.resource(consensusCapsule).get();
            assertThat(capsule).isNotNull();

            List<Pod> pods = kubernetesClient
                    .resources(Pod.class)
                    .inNamespace(namespace)
                    .withLabel(LABEL_NODE_NAME, name) // convention: capsule name is the node name
                    .list()
                    .getItems();
            assertThat(pods.size()).isEqualTo(2); // two pods per node should be running at this stage
            assertThat(pods.getFirst().getStatus()).isNotNull();
            for (Pod pod : pods) {
                assertThat(pod.getStatus().getPhase()).isEqualTo("Running");
                assertThat(pod.getStatus().getPodIP()).isNotEmpty();
            }
        }
    }
}
