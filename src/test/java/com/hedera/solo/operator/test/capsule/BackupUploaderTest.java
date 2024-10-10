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

import com.hedera.solo.operator.model.commonspec.StreamUploader;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.Containers;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.ContainersBuilder;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.Sidecars;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.SidecarsBuilder;
import io.fabric8.kubernetes.api.model.Container;
import io.fabric8.kubernetes.api.model.EnvVar;
import io.fabric8.kubernetes.api.model.Pod;
import io.quarkus.test.junit.QuarkusTest;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.concurrent.TimeUnit;
import org.junit.jupiter.api.Test;

@QuarkusTest
public class BackupUploaderTest extends ConsensusCapsuleTestAbstract {
    public static final String TEMPLATE = "containers/backup-uploader.yaml";

    @Override
    @Test
    void testSingleNodeDeployment() throws Exception {
        super.testSingleNodeDeployment();
        this.checkBackupUploader(1);
    }

    @Override
    @Test
    void testMultiNodeDeployment() throws Exception {
        super.testMultiNodeDeployment();
        this.checkBackupUploader(3);
    }

    @Override
    protected Containers prepareConsensusPodContainers() {
        StreamUploader uploader = loadYaml(StreamUploader.class, getClass(), TEMPLATE);
        return new ContainersBuilder().withBackupUploader(uploader).build();
    }

    private void checkBackupUploader(final int total) {
        await().atMost(30, TimeUnit.SECONDS).untilAsserted(() -> {
            List<Pod> consensusPods = fetchConsensusPods();
            assertThat(consensusPods).hasSize(total);

            for (Pod p : consensusPods) {
                String accountId = p.getMetadata().getLabels().get(LABEL_ACCOUNT_ID);
                assertThat(accountId).isNotEmpty();

                for (Container c : p.getSpec().getContainers()) {
                    if (c.getName().equals("backup-uploader")) {
                        Map<String, String> envMap = new HashMap<>();
                        for (EnvVar env : c.getEnv()) {
                            envMap.put(env.getName(), env.getValue());
                        }

                        assertThat(envMap.get("RCLONE_S3_ACCESS_KEY_ID")).isNotEmpty();
                        assertThat(envMap.get("RCLONE_S3_ENDPOINT")).isNotEmpty();
                        assertThat(envMap.get("BACKUP_UPLOADER_BUCKET_1")).isNotEmpty();

                        break;
                    }
                }
            }
        });
    }
}
