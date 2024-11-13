// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.capsule;

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
import io.fabric8.kubernetes.api.model.PersistentVolumeClaim;
import io.fabric8.kubernetes.api.model.PersistentVolumeClaimSpec;
import io.fabric8.kubernetes.api.model.Pod;
import io.fabric8.kubernetes.api.model.Volume;
import io.quarkus.test.junit.QuarkusTest;
import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.TimeUnit;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

@QuarkusTest
public class PersistentVolumeTest extends ConsensusCapsuleTestAbstract {
    protected PersistentVolumeClaimSpec preparePvcSpec(NodeInfo node, String volSuffix) {
        PersistentVolumeClaim pvc =
                loadYaml(PersistentVolumeClaim.class, getClass(), "pvc/%s-pvc.yaml".formatted(volSuffix));
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
    @DisplayName("volume with PVC should succeed for a single-node network")
    void testSingleNodeDeployment() throws Exception {
        super.testSingleNodeDeployment();
        this.check(1);
    }

    @Test
    @DisplayName("volume with PVC should succeed for a multi-node network")
    void testMultiNodeDeployment() throws Exception {
        super.testMultiNodeDeployment();
        this.check(3);
    }

    private void check(int count) {
        await().atMost(30, TimeUnit.SECONDS).untilAsserted(() -> {
            List<Pod> consensusPods = fetchConsensusPods();
            assertThat(consensusPods).hasSize(count);

            for (Pod p : consensusPods) {
                assertThat(p.getSpec().getVolumes().size()).isGreaterThanOrEqualTo(VOLUME_MOUNT_NAMES.size());
                List<Volume> pvcs = new ArrayList<>();
                p.getSpec().getVolumes().stream()
                        .filter(v ->
                                VOLUME_MOUNT_NAMES.containsValue(v.getName()) && v.getPersistentVolumeClaim() != null)
                        .forEach(pvcs::add);
                assertThat(pvcs).hasSize(VOLUME_MOUNT_NAMES.size());
            }
        });
    }
}
