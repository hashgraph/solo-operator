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
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.podproperties.Containers;
import com.hedera.solo.operator.model.v1alpha1.networkoperationspec.NetworkUpgrade;
import com.hedera.solo.operator.reconciler.common.Constants;
import com.hedera.solo.operator.reconciler.common.Metadata;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.tasks.ExecuteImmediateTask;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.tasks.NowFrozenTask;
import com.hedera.solo.operator.test.fixtures.Resources;
import com.hedera.solo.operator.test.utils.TemplateLoader;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.Pod;
import io.fabric8.kubernetes.api.model.PodCondition;
import io.quarkus.test.junit.QuarkusTest;
import io.quarkus.test.junit.QuarkusTestProfile;
import io.quarkus.test.junit.TestProfile;
import java.time.Duration;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.concurrent.TimeUnit;
import java.util.function.Function;
import org.junit.jupiter.api.Test;

@QuarkusTest
@TestProfile(NetworkUpgradeOperationTest.class)
public class NetworkUpgradeOperationTest extends ConsensusCapsuleTestAbstract implements QuarkusTestProfile {

    @Override
    void cleanup() {
        cleanupNetworkOperations();
        cleanupConsensusCapsules();
        cleanupOrbit();
    }

    void cleanupNetworkOperations() {
        List<NetworkOperation> items = kubernetesClient
                .resources(NetworkOperation.class)
                .inNamespace(namespace)
                .list()
                .getItems();

        for (NetworkOperation item : items) {
            // delete the item
            kubernetesClient.resource(item).delete();
        }

        await().atMost(Duration.ofSeconds(30)).untilAsserted(() -> {
            assertThat(kubernetesClient
                            .resources(NetworkOperation.class)
                            .inNamespace(namespace)
                            .list()
                            .getItems())
                    .isEmpty();
        });
    }

    void cleanupConsensusCapsules() {
        List<ConsensusCapsule> items = kubernetesClient
                .resources(ConsensusCapsule.class)
                .inNamespace(namespace)
                .list()
                .getItems();

        for (ConsensusCapsule item : items) {
            // delete the item
            kubernetesClient.resource(item).delete();
        }

        await().atMost(Duration.ofSeconds(30)).untilAsserted(() -> {
            assertThat(kubernetesClient
                            .resources(ConsensusCapsule.class)
                            .inNamespace(namespace)
                            .list()
                            .getItems())
                    .isEmpty();
        });
    }

    void cleanupOrbit() {
        Resources.removeByName(kubernetesClient, Orbit.class, namespace);
        await().atMost(Duration.ofSeconds(120))
                .until(() -> kubernetesClient.namespaces().withName(namespace).get() == null);
    }

    @Override
    public Map<String, String> getConfigOverrides() {
        return Map.of(
                "solo.operator.network-operation.task.check.max-retries", "60",
                "solo.operator.network-operation.task.check.retry-interval", "5");
    }

    @Override
    @Test
    void testSingleNodeDeployment() throws Exception {
        final List<ConsensusCapsule> consensusCapsules = this.deployNetwork(1);
        final NetworkOperation upgrade = triggerExecuteImmediate(consensusCapsules);
        triggerNowFrozen(upgrade, consensusCapsules);
    }

    @Override
    @Test
    void testMultiNodeDeployment() throws Exception {
        final List<ConsensusCapsule> consensusCapsules = this.deployNetwork(3);
        NetworkOperation upgrade = triggerExecuteImmediate(consensusCapsules);
        upgrade = triggerFreezeScheduled(upgrade, consensusCapsules);
        triggerNowFrozen(upgrade, consensusCapsules);
    }

    @Override
    protected Containers prepareConsensusPodContainers() {
        final StreamUploader rsu =
                TemplateLoader.load(StreamUploader.class, TemplateLoader.TEMPLATE_RECORD_STREAM_UPLOADER);
        rsu.getSoftwareVersion().setImageTag("1.3.0"); // force an older version to test upgrade

        final StreamUploader esu =
                TemplateLoader.load(StreamUploader.class, TemplateLoader.TEMPLATE_EVENTS_STREAM_UPLOADER);
        esu.getSoftwareVersion().setImageTag("1.3.0"); // force an older version to test upgrade

        final StreamUploader rssu =
                TemplateLoader.load(StreamUploader.class, TemplateLoader.TEMPLATE_RECORD_STREAM_SIDECAR_UPLOADER);
        esu.getSoftwareVersion().setImageTag("1.3.0"); // force an older version to test upgrade

        final StreamUploader bu = TemplateLoader.load(StreamUploader.class, TemplateLoader.TEMPLATE_BACKUP_UPLOADER);
        bu.getSoftwareVersion().setImageTag("0.5.0"); // force an older version to test upgrade

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

    protected List<AddressBookEntry> prepareAddressBook(List<ConsensusCapsule> consensusCapsules) {
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

        return addressBookEntries;
    }

    protected NetworkOperation prepareNetworkUpgrade(
            NetworkOperation netOp,
            final NetworkUpgrade.WorkflowStep workflowStep,
            List<AddressBookEntry> addressBookEntries) {

        final NetworkOperation updated = edit(netOp)
                .with(metadata -> metadata.editMetadata()
                        .withName("network-upgrade")
                        .withNamespace(namespace)
                        .withLabels(Map.of(LABEL_K8S_MANAGED_BY, SOLO_OPERATOR)))
                .with(spec -> spec.editSpec()
                        .withMode(NetworkOperationSpec.Mode.NetworkUpgrade)
                        .withOrbit(namespace)
                        .editNetworkUpgrade()
                        .withWorkflowStep(workflowStep))
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
                                        .withImageTag("1.4.0"))
                                .build())
                        .withImageVerificationSpec(edit(new ImageVerificationSpec())
                                .with(
                                        ivs -> ivs.withOs(ImageVerificationSpec.Os.linux)
                                                .withArchitecture(ImageVerificationSpec.Architecture.arm64)
                                                .withLayerHashes(
                                                        "sha256:41af1b5f0f51947706ae712999cf098bef968a7799e7cb4bb2268829e62a6ab3",
                                                        "sha256:193a17f443d6422e13871b179031baab5e896b4ed1857ef5818ed4fb7c9c41fc",
                                                        "sha256:0be5c6c9871eafb3bd07bd309795db4889fb1bc912ec5f2d9bdbc3d14b655704",
                                                        "sha256:5b57223652278c4a0310a61224eca2c97bba0708deb14549adb96745274cb51d",
                                                        "sha256:8603da644143cf0815ecedb23437eaafea6abad8f51274fe0b4727e5484068a8",
                                                        "sha256:422c9b8842577fbd9907165ad4e87d372cc41573c83283f037f75c8fca576882",
                                                        "sha256:e68d61a0fe56df44ba661ac578879befcce6b3c74a8198bae4967ca202cba21a"))
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
                                                        "sha256:0c5227665c11379f79e9da3d3e4f1724f9316b87d259ac0131628ca1b923a392",
                                                        "sha256:b2d3390297ee0817aedbebad2700de263fbde157515977289a16ff519a86aa99",
                                                        "sha256:e7ef608bf9c73ab2dd1ac0ddeaf81226648b8769f86dad9ab4fc2daab339be5b",
                                                        "sha256:277eaa6804a9ae6150517c6354df42f91c8ab0e7f1f7c9e8adb15cb0f407afe0",
                                                        "sha256:ca97ca0caed60355299d86926a4636b0bbe5aa627f387508b6985b1659b7a2f9",
                                                        "sha256:2f57474a27b1f8ef3d4bc0023d94fd327862260e59d5b94e459c8681d9189289",
                                                        "sha256:19273e2e2728c7b3d49fdb7099bd169ac1da367ea05fdf6b903fc71ed54d44d8",
                                                        "sha256:885d7bc6dabe6f506b816e4abf57beb8ab915dc4cfabf9b2de883df78f7f4959",
                                                        "sha256:50bc7a11b84ccfb4393e048dad575e96b417b1b6593a4013500d742f9a87dad9",
                                                        "sha256:4eb4862da3dbe5fb513cc1ab7ace5fa35513f22b80ea8d71c5b05764132cb232",
                                                        "sha256:e56ad9bcbe3b8f40784043c81ef33d59da8bca3e2323c5662dc53fd80a422cb1"))
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

        // only add it once during ExecuteImmediate
        if (workflowStep.equals(NetworkUpgrade.WorkflowStep.ExecuteImmediate)) {
            final Orbit orbit = kubernetesClient
                    .resources(Orbit.class)
                    .inNamespace(namespace)
                    .list()
                    .getItems()
                    .getFirst();
            assertThat(orbit).isNotNull();

            final Map<String, String> newNetworkSettings = new HashMap<>();
            if (orbit.getSpec().getConsensus().getNetworkSettings() != null) {
                newNetworkSettings.putAll(orbit.getSpec().getConsensus().getNetworkSettings());
            }
            newNetworkSettings.put("test1", "1");

            final Map<String, String> newGenesisProperty = new HashMap<>();
            if (orbit.getSpec().getConsensus().getGenesisProperties() != null) {
                newGenesisProperty.putAll(orbit.getSpec().getConsensus().getGenesisProperties());
            }
            newGenesisProperty.put("test2", "2");

            final Map<String, String> newApiPermission = new HashMap<>();
            if (orbit.getSpec().getConsensus().getPermissionConfiguration() != null) {
                newApiPermission.putAll(orbit.getSpec().getConsensus().getPermissionConfiguration());
            }
            newApiPermission.put("test3", "3");

            final Map<String, String> newApplicationProperties = new HashMap<>();
            if (orbit.getSpec().getConsensus().getApplicationConfiguration() != null) {
                newApplicationProperties.putAll(orbit.getSpec().getConsensus().getApplicationConfiguration());
            }
            newApplicationProperties.put("test4", "4");
            return edit(updated)
                    .with(s -> s.editSpec()
                            .editNetworkUpgrade()
                            .editConfigurationFiles()
                            .withNetworkSettings(newNetworkSettings)
                            .withGenesisProperties(newGenesisProperty)
                            .withApplicationProperties(newApplicationProperties)
                            .withPermissionConfiguration(newApiPermission)
                            .endConfigurationFiles())
                    .build();
        }

        return updated;
    }

    protected void checkNowFrozenStatus(
            List<ConsensusCapsule> consensusCapsules, Function<NetworkOperation, Boolean> isDone) {
        await().atMost(5, TimeUnit.MINUTES).pollDelay(Duration.ofSeconds(2)).untilAsserted(() -> {
            NetworkOperation netOp = kubernetesClient
                    .resources(NetworkOperation.class)
                    .inNamespace(namespace)
                    .withName("network-upgrade")
                    .get();
            assertThat(netOp).isNotNull();
            assertThat(netOp.getStatus()).isNotNull();
            assertThat(isDone.apply(netOp)).isTrue();
            assertThat(netOp.getStatus().getSoftwareSourceStatus()).isNotNull();
            assertThat(netOp.getStatus().getSoftwareSourceStatus().getConsensusNode())
                    .isNotNull();
            assertThat(netOp.getStatus().getUpgradedNodes().size()).isEqualTo(consensusCapsules.size());
            assertThat(netOp.getStatus().getPhase()).isEqualTo(NetworkOperationStatus.Phase.Running);

            ConfigMap cc = kubernetesClient
                    .resources(ConfigMap.class)
                    .inNamespace(namespace)
                    .withName(Constants.CLUSTER_CONFIG_NAME)
                    .get();
            assertThat(cc).isNotNull();
        });

        await().atMost(5, TimeUnit.MINUTES).pollDelay(Duration.ofSeconds(2)).untilAsserted(() -> {
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
                for (Pod pod : pods) {
                    assertThat(pod.getStatus()).isNotNull();
                    assertThat(pod.getMetadata().getDeletionTimestamp()).isNull();
                    Optional<PodCondition> ready = pod.getStatus().getConditions().stream()
                            .filter(c -> "Ready".equals(c.getType()))
                            .findAny();
                    assertThat(ready).isPresent();
                    assertThat(ready.get().getStatus()).isEqualTo("True");
                    assertThat(pod.getStatus().getPodIP()).isNotEmpty();
                }
            }
        });

        // wait for the network operation to complete
        await().atMost(3, TimeUnit.MINUTES).pollDelay(Duration.ofSeconds(2)).untilAsserted(() -> {
            NetworkOperation netOp = kubernetesClient
                    .resources(NetworkOperation.class)
                    .inNamespace(namespace)
                    .withName("network-upgrade")
                    .get();
            assertThat(netOp).isNotNull();
            assertThat(netOp.getStatus().getPhase()).isEqualTo(NetworkOperationStatus.Phase.Succeeded);
        });
    }

    protected NetworkOperation triggerExecuteImmediate(List<ConsensusCapsule> consensusCapsules) {
        final List<AddressBookEntry> addressBookEntries = prepareAddressBook(consensusCapsules);
        final NetworkOperation executeImmediate = prepareNetworkUpgrade(
                new NetworkOperation(), NetworkUpgrade.WorkflowStep.ExecuteImmediate, addressBookEntries);
        kubernetesClient.resource(executeImmediate).create();

        await().atMost(2, TimeUnit.MINUTES).pollDelay(Duration.ofSeconds(2)).untilAsserted(() -> {
            NetworkOperation netOp = kubernetesClient
                    .resources(NetworkOperation.class)
                    .inNamespace(namespace)
                    .withName("network-upgrade")
                    .get();
            assertThat(netOp).isNotNull();
            assertThat(netOp.getStatus()).isNotNull();
            assertThat(netOp.getStatus().getSoftwareSourceStatus()).isNotNull();
            assertThat(netOp.getStatus().getSoftwareSourceStatus().getConsensusNode())
                    .isNotNull();
            assertThat(netOp.getStatus().getPhase()).isNotEqualTo(NetworkOperationStatus.Phase.Succeeded);
            assertThat(ExecuteImmediateTask.isDone(netOp)).isTrue();
        });

        return executeImmediate;
    }

    protected NetworkOperation triggerFreezeScheduled(
            final NetworkOperation upgrade, List<ConsensusCapsule> consensusCapsules) {
        final List<AddressBookEntry> addressBookEntries = prepareAddressBook(consensusCapsules);
        final NetworkOperation freezeScheduled =
                prepareNetworkUpgrade(upgrade, NetworkUpgrade.WorkflowStep.FreezeScheduled, addressBookEntries);
        kubernetesClient.resource(freezeScheduled).patch();

        await().atMost(2, TimeUnit.MINUTES).pollDelay(Duration.ofSeconds(2)).untilAsserted(() -> {
            NetworkOperation netOp = kubernetesClient
                    .resources(NetworkOperation.class)
                    .inNamespace(namespace)
                    .withName("network-upgrade")
                    .get();
            assertThat(netOp).isNotNull();
            assertThat(netOp.getStatus()).isNotNull();
            assertThat(netOp.getStatus().getSoftwareSourceStatus()).isNotNull();
            assertThat(netOp.getStatus().getSoftwareSourceStatus().getConsensusNode())
                    .isNotNull();
            assertThat(netOp.getStatus().getPhase()).isNotEqualTo(NetworkOperationStatus.Phase.Succeeded);
            assertThat(ExecuteImmediateTask.isDone(netOp)).isTrue();
        });

        return freezeScheduled;
    }

    protected void triggerNowFrozen(NetworkOperation upgrade, List<ConsensusCapsule> consensusCapsules) {
        final List<AddressBookEntry> addressBookEntries = prepareAddressBook(consensusCapsules);
        final NetworkOperation nowFrozen =
                prepareNetworkUpgrade(upgrade, NetworkUpgrade.WorkflowStep.NowFrozen, addressBookEntries);
        kubernetesClient.resource(nowFrozen).patch();
        checkNowFrozenStatus(consensusCapsules, NowFrozenTask::isDone);
    }
}
