// SPDX-License-Identifier: Apache-2.0

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
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.podproperties.Volumes;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.podproperties.VolumesBuilder;
import io.fabric8.kubernetes.api.model.PersistentVolumeClaim;
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
    private String pvcClaimName(NodeInfo node, String volSuffix) {
        return "%s-%s".formatted(node.name(), volSuffix);
    }

    protected Volume prepareVolumeWithClaimSource(NodeInfo node, String volSuffix) {
        final PersistentVolumeClaimVolumeSource pvc = new PersistentVolumeClaimVolumeSourceBuilder()
                .withClaimName(pvcClaimName(node, volSuffix))
                .build();

        return new VolumeBuilder().withPersistentVolumeClaim(pvc).build();
    }

    @Override
    protected Volumes prepareCapsulePodVolumes(NodeInfo node) {
        return new VolumesBuilder()
                .withLogs(prepareVolumeWithClaimSource(node, VOLUME_LOGS_NAME_SUFFIX))
                .withStats(prepareVolumeWithClaimSource(node, VOLUME_STATS_NAME_SUFFIX))
                .withStates(prepareVolumeWithClaimSource(node, VOLUME_STATES_NAME_SUFFIX))
                .withUpgrade(prepareVolumeWithClaimSource(node, VOLUME_UPGRADE_NAME_SUFFIX))
                .withRecords(prepareVolumeWithClaimSource(node, VOLUME_RECORD_STREAM_NAME_SUFFIX))
                .withEvents(prepareVolumeWithClaimSource(node, VOLUME_EVENTS_STREAM_NAME_SUFFIX))
                .build();
    }

    protected PersistentVolumeClaim prepareInputPvc(NodeInfo node, String volSuffix) {
        PersistentVolumeClaim pvc = loadYaml(
                        PersistentVolumeClaim.class, getClass(), "pvc/%s-pvc.yaml".formatted(volSuffix))
                .edit()
                .editMetadata()
                .withName(pvcClaimName(node, volSuffix))
                .withNamespace(namespace)
                .addToLabels("consensus-capsule", node.name())
                .endMetadata()
                .build();
        return pvc;
    }

    @Override
    protected PersistentVolumeClaims prepareCapsulePVCs(NodeInfo node) {
        return new PersistentVolumeClaimsBuilder()
                .withLogs(prepareInputPvc(node, VOLUME_LOGS_NAME_SUFFIX))
                .withStats(prepareInputPvc(node, VOLUME_STATS_NAME_SUFFIX))
                .withStates(prepareInputPvc(node, VOLUME_STATES_NAME_SUFFIX))
                .withUpgrade(prepareInputPvc(node, VOLUME_UPGRADE_NAME_SUFFIX))
                .withRecords(prepareInputPvc(node, VOLUME_RECORD_STREAM_NAME_SUFFIX))
                .withEvents(prepareInputPvc(node, VOLUME_EVENTS_STREAM_NAME_SUFFIX))
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
