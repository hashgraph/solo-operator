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
import com.hedera.solo.operator.model.commonspec.OpenTelemetryCollector;
import com.hedera.solo.operator.model.commonspec.StreamUploader;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationSpec;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.podproperties.Containers;
import com.hedera.solo.operator.model.v1alpha1.networkoperationspec.NetworkUpgrade;
import com.hedera.solo.operator.reconciler.common.Constants;
import com.hedera.solo.operator.reconciler.common.Metadata;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.tasks.NowFrozenTask;
import com.hedera.solo.operator.test.utils.TemplateLoader;
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

    @Override
    protected Containers prepareConsensusPodContainers() {
        final StreamUploader rsu =
                TemplateLoader.load(StreamUploader.class, TemplateLoader.TEMPLATE_RECORD_STREAM_UPLOADER);
        rsu.getSoftwareVersion().setImageTag("1.3.0"); // force an older version to test upgrade

        final StreamUploader esu =
                TemplateLoader.load(StreamUploader.class, TemplateLoader.TEMPLATE_EVENTS_STREAM_UPLOADER);
        esu.getSoftwareVersion().setImageTag("1.3.0"); // force an older version to test upgrade

        final StreamUploader bu = TemplateLoader.load(StreamUploader.class, TemplateLoader.TEMPLATE_BACKUP_UPLOADER);
        bu.getSoftwareVersion().setImageTag("0.5.0"); // force an older version to test upgrade

        final StreamUploader rssu =
                TemplateLoader.load(StreamUploader.class, TemplateLoader.TEMPLATE_RECORD_STREAM_SIDECAR_UPLOADER);
        esu.getSoftwareVersion().setImageTag("1.3.0"); // force an older version to test upgrade

        final OpenTelemetryCollector oc =
                TemplateLoader.load(OpenTelemetryCollector.class, TemplateLoader.TEMPLATE_OPEN_TELEMETRY_COLLECTOR);
        oc.getSoftwareVersion().setImageTag("0.72.0"); // force an older version to test upgrade

        return getContainersBuilder()
                .withRecordStreamUploader(rsu)
                .withEventsStreamUploader(esu)
                .withBackupUploader(bu)
                .withRecordStreamSidecarFilesUploader(rssu)
                .withOpenTelemetryCollector(oc)
                .build();
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
                        .withWorkflowStep(NetworkUpgrade.WorkflowStep.NowFrozen))
                .with(spec -> spec.editSpec()
                        .editNetworkUpgrade()
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
                .with(spec -> spec.editSpec()
                        .editNetworkUpgrade()
                        .editChangesets()
                        .editRecordStreamUploaderChangeset()
                        .editSoftwareVersionSource()
                        .withImageRepositories(edit(new ImageRepository())
                                .with(repo -> repo.withRepository("gcr.io/hedera-registry")
                                        .withImageName("uploader-mirror")
                                        .withImageTag("2.1.0"))
                                .build())
                        .withImageVerificationSpec(edit(new ImageVerificationSpec())
                                .with(
                                        ivs -> ivs.withOs(ImageVerificationSpec.Os.linux)
                                                .withArchitecture(ImageVerificationSpec.Architecture.arm64)
                                                .withLayerHashes(
                                                        "sha256:7c457f213c7634afb95a0fb2410a74b7b5bc0ba527033362c240c7a11bef4331",
                                                        "sha256:e2b533a6dd6418e4a6992797fe986c6c2790a8031485db783becac9d46b7bf26",
                                                        "sha256:b3ccb45587f906c70741c096867e075206fc95806db2dff458578b95f9489cc4",
                                                        "sha256:e7a7e1f61944adc64431a2ed848dbea51b894d94c4845bf27b4e53732286d81f",
                                                        "sha256:d3d1a9788837b3541e94b4e128d9f20ca06ea51e76491aea3d487476ed4b4916",
                                                        "sha256:1ee13fa23607d2733265b4b88b9ac457c550eb9c49261180c870884a1c27a3da"))
                                .build()))
                .with(spec -> spec.editSpec()
                        .editNetworkUpgrade()
                        .editChangesets()
                        .editBackupUploaderChangeset()
                        .editSoftwareVersionSource()
                        .withImageRepositories(edit(new ImageRepository())
                                .with(repo -> repo.withRepository("gcr.io/hedera-registry")
                                        .withImageName("hedera-backups")
                                        .withImageTag("0.6.0"))
                                .build())
                        .withImageVerificationSpec(edit(new ImageVerificationSpec())
                                .with(
                                        ivs -> ivs.withOs(ImageVerificationSpec.Os.linux)
                                                .withArchitecture(ImageVerificationSpec.Architecture.arm64)
                                                .withLayerHashes(
                                                        "sha256:93e7f5ee2426cbcb790966dccc80b8573db712a8532fc4fbf3ec66205ac7492b",
                                                        "sha256:83d2a6a35540a05e7cf9957245423c8f8087fd7555864fbd219c1a7a84386660",
                                                        "sha256:745cce1e22a98215b6b7975ec95ff342871b4dcb5fe07b57180152b1075df64c"))
                                .build()))
                .with(spec -> spec.editSpec()
                        .editNetworkUpgrade()
                        .editChangesets()
                        .editOtelCollector()
                        .editSoftwareVersionSource()
                        .withImageRepositories(edit(new ImageRepository())
                                .with(repo -> repo.withRepository("docker.io/otel")
                                        .withImageName("opentelemetry-collector-contrib")
                                        .withImageTag("0.72.0"))
                                .build())
                        .withImageVerificationSpec(edit(new ImageVerificationSpec())
                                .with(
                                        ivs -> ivs.withOs(ImageVerificationSpec.Os.linux)
                                                .withArchitecture(ImageVerificationSpec.Architecture.arm64)
                                                .withLayerHashes(
                                                        "sha256:048507c36033a60333eafcc13166632b4f9df12aa88ecf37e097292ddd3032c4",
                                                        "sha256:4e4e3f1376c1d16b3ffe7fc3309364b0efa6dbebd4b4b2148d93af80f729b4ee",
                                                        "sha256:474af8540d2c99efbcba4abfe8813d130745d277122235280c09c85266366796"))
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
        });
    }
}
