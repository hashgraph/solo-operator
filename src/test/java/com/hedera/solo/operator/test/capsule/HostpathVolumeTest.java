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
import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.pod.Volumes;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.pod.VolumesBuilder;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.pod.volumes.StreamsBuilder;
import io.fabric8.kubernetes.api.model.HostPathVolumeSource;
import io.fabric8.kubernetes.api.model.HostPathVolumeSourceBuilder;
import io.fabric8.kubernetes.api.model.Pod;
import io.fabric8.kubernetes.api.model.Volume;
import io.fabric8.kubernetes.api.model.VolumeBuilder;
import io.quarkus.test.junit.QuarkusTest;
import java.util.List;
import java.util.Map;
import java.util.concurrent.TimeUnit;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

@QuarkusTest
public class HostpathVolumeTest extends ConsensusCapsuleTestAbstract {

    protected static final Map<String, String> hostPaths = Map.of(
            VOLUME_LOGS_NAME_SUFFIX, "/tmp/%s/volumes/logs",
            VOLUME_STATS_NAME_SUFFIX, "/tmp/%s/volumes/stats",
            VOLUME_STATES_NAME_SUFFIX, "/tmp/%s//volumes/saved",
            VOLUME_UPGRADE_NAME_SUFFIX, "/tmp/%s/volumes/upgrade",
            VOLUME_RECORD_STREAM_NAME_SUFFIX, "/tmp/%s/volumes/recordStream",
            VOLUME_EVENTS_STREAM_NAME_SUFFIX, "/tmp/%s/volumes/eventsStreams");

    protected Volume prepareNodeVolumeWithHostpath(NodeInfo node, String volSuffix) {
        final HostPathVolumeSource hs = new HostPathVolumeSourceBuilder()
                .withPath(hostPaths.get(volSuffix).formatted(node.name()))
                .withType("DirectoryOrCreate")
                .build();

        return new VolumeBuilder().withHostPath(hs).build();
    }

    @Override
    protected Volumes prepareCapsulePodVolumes(NodeInfo node) {
        return new VolumesBuilder()
                .withLogs(prepareNodeVolumeWithHostpath(node, VOLUME_LOGS_NAME_SUFFIX))
                .withStats(prepareNodeVolumeWithHostpath(node, VOLUME_STATS_NAME_SUFFIX))
                .withStates(prepareNodeVolumeWithHostpath(node, VOLUME_STATES_NAME_SUFFIX))
                .withUpgrade(prepareNodeVolumeWithHostpath(node, VOLUME_UPGRADE_NAME_SUFFIX))
                .withStreams(new StreamsBuilder()
                        .withRecord(prepareNodeVolumeWithHostpath(node, VOLUME_RECORD_STREAM_NAME_SUFFIX))
                        .withEvents(prepareNodeVolumeWithHostpath(node, VOLUME_EVENTS_STREAM_NAME_SUFFIX))
                        .build())
                .build();
    }

    @Test
    @DisplayName("volume with hostpath should succeed for a single-node network")
    void testSingleNodeDeployment() throws Exception {
        super.testSingleNodeDeployment();
        this.check(1);
    }

    @Test
    @DisplayName("volume with hostpath should succeed for a multi-node network")
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
                        assertThat(v.getHostPath()).isNotNull();
                    }
                }
            }
        });
    }
}
