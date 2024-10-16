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

import static com.hedera.solo.operator.reconciler.common.Constants.LABEL_ACCOUNT_ID;
import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;
import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.PersistentVolumeClaims;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.PersistentVolumeClaimsBuilder;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.pod.Volumes;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.pod.VolumesBuilder;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.pod.volumes.StreamsBuilder;
import io.fabric8.kubernetes.api.model.PersistentVolumeClaim;
import io.fabric8.kubernetes.api.model.PersistentVolumeClaimVolumeSourceBuilder;
import io.fabric8.kubernetes.api.model.Pod;
import io.fabric8.kubernetes.api.model.Volume;
import io.fabric8.kubernetes.api.model.VolumeBuilder;
import io.quarkus.test.junit.QuarkusTest;
import java.util.List;
import java.util.concurrent.TimeUnit;
import org.junit.jupiter.api.Test;

@QuarkusTest
public class PersistentVolumeTest extends HostpathVolumeTest {
    protected Volume prepareVolume(NodeInfo node, String volumeName) {
        return new VolumeBuilder()
                .withPersistentVolumeClaim(new PersistentVolumeClaimVolumeSourceBuilder()
                        .withClaimName("%s-%s".formatted(node.name(), volumeName))
                        .build())
                .build();
    }

    protected Volumes prepareCapsulePodVolumes(NodeInfo node) {
        return new VolumesBuilder()
                .withLogs(prepareVolume(node, VOLUME_LOGS_NAME_SUFFIX))
                .withStats(prepareVolume(node, VOLUME_STATS_NAME_SUFFIX))
                .withStates(prepareVolume(node, VOLUME_STATES_NAME_SUFFIX))
                .withUpgrade(prepareVolume(node, VOLUME_UPGRADE_NAME_SUFFIX))
                .withStreams(new StreamsBuilder()
                        .withRecord(prepareVolume(node, VOLUME_RECORD_STREAM_NAME_SUFFIX))
                        .withEvents(prepareVolume(node, VOLUME_EVENTS_STREAM_NAME_SUFFIX))
                        .build())
                .build();
    }

    protected PersistentVolumeClaim preparePersistentVolume(NodeInfo node, String volumeName) {
        return loadYaml(PersistentVolumeClaim.class, getClass(), "pvc/%s-pvc.yaml".formatted(volumeName))
                .edit()
                .editMetadata()
                .withName("%s-%s".formatted(node.name(), volumeName))
                .withNamespace(namespace)
                .addToLabels("consensus-capsule", node.name())
                .endMetadata()
                .build();
    }

    protected PersistentVolumeClaims prepareCapsulePVCs(NodeInfo node) {
        return new PersistentVolumeClaimsBuilder()
                .withLogs(preparePersistentVolume(node, VOLUME_LOGS_NAME_SUFFIX).getSpec())
                .withStats(
                        preparePersistentVolume(node, VOLUME_STATS_NAME_SUFFIX).getSpec())
                .withStates(
                        preparePersistentVolume(node, VOLUME_STATES_NAME_SUFFIX).getSpec())
                .withStreams(
                        new com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.persistentvolumeclaims
                                        .StreamsBuilder()
                                .withRecord(preparePersistentVolume(node, VOLUME_RECORD_STREAM_NAME_SUFFIX)
                                        .getSpec())
                                .withEvents(preparePersistentVolume(node, VOLUME_EVENTS_STREAM_NAME_SUFFIX)
                                        .getSpec())
                                .build())
                .build();
    }

    //    private void setupPersistentVolumeClaims(List<NodeInfo> nodes) {
    //        for (final String pvcDirName : List.of(
    //                "output",
    //                "data-keys",
    //                "data-saved",
    //                "data-stats",
    //                "data-upgrade",
    //                "events-streams",
    //                "record-streams")) {
    //            for (NodeInfo node : nodes) {
    //                final String pvcFile = String.format("pvc/%s-pvc.yaml", pvcDirName);
    //                kubernetesClient
    //                        .resource(loadYaml(PersistentVolumeClaim.class, getClass(), pvcFile)
    //                                .edit()
    //                                .editMetadata()
    //                                .withName("%s-%s".formatted(node.name(), pvcDirName))
    //                                .withNamespace(namespace)
    //                                .addToLabels("consensus-capsule", node.name())
    //                                .endMetadata()
    //                                .build())
    //                        .create();
    //            }
    //        }
    //    }

    @Test
    void testVolumesWithPVCs() throws Exception {
        super.testSingleNodeDeployment();
        this.check();
    }

    // failure case: both hostpath and PVCs are specified
    @Test
    void testVolumesWithPVCAndHostpath() throws Exception {
        super.testSingleNodeDeployment();
    }

    private void check() {
        await().atMost(30, TimeUnit.SECONDS).untilAsserted(() -> {
            List<Pod> consensusPods = fetchConsensusPods();
            assertThat(consensusPods).hasSize(1);

            for (Pod p : consensusPods) {
                String accountId = p.getMetadata().getLabels().get(LABEL_ACCOUNT_ID);
                assertThat(accountId).isNotEmpty();
                assertThat(p.getSpec().getVolumes().size()).isGreaterThanOrEqualTo(6);

                // TODO check for specific volumes
            }
        });
    }
}
