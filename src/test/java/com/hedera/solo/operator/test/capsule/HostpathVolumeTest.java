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
import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

import io.fabric8.kubernetes.api.model.Pod;
import io.quarkus.test.junit.QuarkusTest;
import java.util.List;
import java.util.concurrent.TimeUnit;
import org.junit.jupiter.api.Test;

@QuarkusTest
public class HostpathVolumeTest extends ConsensusCapsuleTestAbstract {
    public static final String VOLUME_LOGS_NAME_SUFFIX = "output";
    public static final String VOLUME_STATS_NAME_SUFFIX = "data_stats";
    public static final String VOLUME_STATES_NAME_SUFFIX = "data_states";
    public static final String VOLUME_UPGRADE_NAME_SUFFIX = "data_upgrade";
    public static final String VOLUME_RECORD_STREAM_NAME_SUFFIX = "record-streams";
    public static final String VOLUME_EVENTS_STREAM_NAME_SUFFIX = "events-streams";

    @Test
    void testVolumesWithHostpath() throws Exception {
        super.testSingleNodeDeployment();
        this.check(1);
    }

    @Test
    void testVolumesWithHostpathMultiNode() throws Exception {
        super.testMultiNodeDeployment();
        this.check(3);
    }

    private void check(int total) {
        await().atMost(30, TimeUnit.SECONDS).untilAsserted(() -> {
            List<Pod> consensusPods = fetchConsensusPods();
            assertThat(consensusPods).hasSize(total);

            for (Pod p : consensusPods) {
                String accountId = p.getMetadata().getLabels().get(LABEL_ACCOUNT_ID);
                assertThat(accountId).isNotEmpty();
                assertThat(p.getSpec().getVolumes().size()).isGreaterThanOrEqualTo(6);

                // TODO check for specific volumes
            }
        });
    }
}
