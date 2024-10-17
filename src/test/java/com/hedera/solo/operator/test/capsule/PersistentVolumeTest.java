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
import static com.hedera.solo.operator.reconciler.common.Constants.VOLUME_EVENTS_STREAM_NAME_SUFFIX;
import static com.hedera.solo.operator.reconciler.common.Constants.VOLUME_LOGS_NAME_SUFFIX;
import static com.hedera.solo.operator.reconciler.common.Constants.VOLUME_MOUNT_NAMES;
import static com.hedera.solo.operator.reconciler.common.Constants.VOLUME_RECORD_STREAM_NAME_SUFFIX;
import static com.hedera.solo.operator.reconciler.common.Constants.VOLUME_STATES_NAME_SUFFIX;
import static com.hedera.solo.operator.reconciler.common.Constants.VOLUME_STATS_NAME_SUFFIX;
import static com.hedera.solo.operator.reconciler.common.Constants.VOLUME_UPGRADE_NAME_SUFFIX;
import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;
import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.PersistentVolumeClaims;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.PersistentVolumeClaimsBuilder;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.pod.Volumes;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.pod.VolumesBuilder;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.pod.volumes.StreamsBuilder;
import io.fabric8.kubernetes.api.model.PersistentVolumeClaim;
import io.fabric8.kubernetes.api.model.PersistentVolumeClaimSpec;
import io.fabric8.kubernetes.api.model.PersistentVolumeClaimVolumeSource;
import io.fabric8.kubernetes.api.model.PersistentVolumeClaimVolumeSourceBuilder;
import io.fabric8.kubernetes.api.model.Pod;
import io.fabric8.kubernetes.api.model.Volume;
import io.fabric8.kubernetes.api.model.VolumeBuilder;
import io.quarkus.test.junit.QuarkusTest;
import java.util.List;
import java.util.concurrent.TimeUnit;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

@QuarkusTest
public class PersistentVolumeTest extends ConsensusCapsuleTestAbstract {
    protected Volume prepareNodeVolumeWithPvc(NodeInfo node, String volSuffix) {
        final PersistentVolumeClaimVolumeSource ps = new PersistentVolumeClaimVolumeSourceBuilder()
                .withClaimName(nodeVolumeName(node, volSuffix))
                .build();

        return new VolumeBuilder().withPersistentVolumeClaim(ps).build();
    }

    @Override
    protected Volumes prepareCapsuleVolumes(NodeInfo node) {
        return new VolumesBuilder()
                .withLogs(prepareNodeVolumeWithPvc(node, VOLUME_LOGS_NAME_SUFFIX))
                .withStats(prepareNodeVolumeWithPvc(node, VOLUME_STATS_NAME_SUFFIX))
                .withStates(prepareNodeVolumeWithPvc(node, VOLUME_STATES_NAME_SUFFIX))
                .withUpgrade(prepareNodeVolumeWithPvc(node, VOLUME_UPGRADE_NAME_SUFFIX))
                .withStreams(new StreamsBuilder()
                        .withRecord(prepareNodeVolumeWithPvc(node, VOLUME_RECORD_STREAM_NAME_SUFFIX))
                        .withEvents(prepareNodeVolumeWithPvc(node, VOLUME_EVENTS_STREAM_NAME_SUFFIX))
                        .build())
                .build();
    }

    protected PersistentVolumeClaimSpec preparePvcSpec(NodeInfo node, String volSuffix) {
        PersistentVolumeClaim pvc = loadYaml(
                        PersistentVolumeClaim.class, getClass(), "pvc/%s-pvc.yaml".formatted(volSuffix))
                .edit()
                .editMetadata()
                .withName(nodeVolumeName(node, volSuffix))
                .withNamespace(namespace)
                .addToLabels("consensus-capsule", node.name())
                .endMetadata()
                .build();
        return pvc.getSpec();
    }

    @Override
    protected PersistentVolumeClaims prepareCapsulePVCs(NodeInfo node) {
        return new PersistentVolumeClaimsBuilder()
                .withLogs(preparePvcSpec(node, VOLUME_LOGS_NAME_SUFFIX))
                .withStats(preparePvcSpec(node, VOLUME_STATS_NAME_SUFFIX))
                .withStates(preparePvcSpec(node, VOLUME_STATES_NAME_SUFFIX))
                .withUpgrade(preparePvcSpec(node, VOLUME_UPGRADE_NAME_SUFFIX))
                .withStreams(
                        new com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.persistentvolumeclaims
                                        .StreamsBuilder()
                                .withRecord(preparePvcSpec(node, VOLUME_RECORD_STREAM_NAME_SUFFIX))
                                .withEvents(preparePvcSpec(node, VOLUME_EVENTS_STREAM_NAME_SUFFIX))
                                .build())
                .build();
    }

    @Test
    @DisplayName("volume with hostpath should succeed for a single-node network")
    void testSingleNodeDeployment() throws Exception {
        super.testSingleNodeDeployment();
        this.check();
    }

    private void check() {
        await().atMost(30, TimeUnit.SECONDS).untilAsserted(() -> {
            List<Pod> consensusPods = fetchConsensusPods();
            assertThat(consensusPods).hasSize(1);

            for (Pod p : consensusPods) {
                String accountId = p.getMetadata().getLabels().get(LABEL_ACCOUNT_ID);
                assertThat(accountId).isNotEmpty();
                assertThat(p.getSpec().getVolumes().size()).isGreaterThanOrEqualTo(VOLUME_MOUNT_NAMES.size());

                for (Volume v : p.getSpec().getVolumes()) {
                    if (VOLUME_MOUNT_NAMES.containsValue(v.getName())) {
                        assertThat(v.getPersistentVolumeClaim()).isNotNull();
                    }
                }
            }
        });
    }
}
