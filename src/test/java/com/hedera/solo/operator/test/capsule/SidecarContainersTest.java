// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.capsule;

import static com.hedera.solo.operator.reconciler.common.Constants.EVENT_STREAM_BUCKET_PATH_TEMPLATE;
import static com.hedera.solo.operator.reconciler.common.Constants.EVENT_STREAM_EXTENSION;
import static com.hedera.solo.operator.reconciler.common.Constants.EVENT_STREAM_SIG_EXTENSION;
import static com.hedera.solo.operator.reconciler.common.Constants.LABEL_ACCOUNT_ID;
import static com.hedera.solo.operator.reconciler.common.Constants.RECORD_STREAM_BUCKET_PATH_TEMPLATE;
import static com.hedera.solo.operator.reconciler.common.Constants.RECORD_STREAM_EXTENSION;
import static com.hedera.solo.operator.reconciler.common.Constants.RECORD_STREAM_SIG_EXTENSION;
import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

import com.hedera.solo.operator.model.commonspec.StreamUploader;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.podproperties.Containers;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.podproperties.containers.OpenTelemetryCollector;
import com.hedera.solo.operator.test.utils.TemplateLoader;
import io.fabric8.kubernetes.api.model.Container;
import io.fabric8.kubernetes.api.model.EnvVar;
import io.fabric8.kubernetes.api.model.Pod;
import io.quarkus.test.junit.QuarkusTest;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.concurrent.TimeUnit;
import org.junit.jupiter.api.Test;

/** This class contains tests for sidecar containers in a consensus capsule deployment. */
@QuarkusTest
public class SidecarContainersTest extends ConsensusCapsuleTestAbstract {
    /**
     * Tests the deployment of a single node and checks the sidecar containers.
     *
     * @throws Exception if an error occurs during the test.
     */
    @Override
    @Test
    void testSingleNodeDeployment() throws Exception {
        super.testSingleNodeDeployment();
        this.checkSidecarContainers(1);
    }

    /**
     * Tests the deployment of multiple nodes and checks the sidecar containers.
     *
     * @throws Exception if an error occurs during the test.
     */
    @Override
    @Test
    void testMultiNodeDeployment() throws Exception {
        super.testMultiNodeDeployment();
        this.checkSidecarContainers(3);
    }

    /**
     * Prepares the containers for a consensus pod based on templates.
     *
     * @return Containers object containing sidecar containers.
     */
    @Override
    protected Containers prepareConsensusPodContainers() {
        final StreamUploader rsu =
                TemplateLoader.load(StreamUploader.class, TemplateLoader.TEMPLATE_RECORD_STREAM_UPLOADER);
        final StreamUploader esu =
                TemplateLoader.load(StreamUploader.class, TemplateLoader.TEMPLATE_EVENTS_STREAM_UPLOADER);
        final StreamUploader bu = TemplateLoader.load(StreamUploader.class, TemplateLoader.TEMPLATE_BACKUP_UPLOADER);
        final StreamUploader rssu =
                TemplateLoader.load(StreamUploader.class, TemplateLoader.TEMPLATE_RECORD_STREAM_SIDECAR_UPLOADER);
        final OpenTelemetryCollector oc =
                TemplateLoader.load(OpenTelemetryCollector.class, TemplateLoader.TEMPLATE_OPEN_TELEMETRY_COLLECTOR);
        return getContainersBuilder()
                .withRecordStreamUploader(rsu)
                .withEventsStreamUploader(esu)
                .withBackupUploader(bu)
                .withRecordStreamSidecarFilesUploader(rssu)
                .withOpenTelemetryCollector(oc)
                .build();
    }

    /**
     * Checks the sidecar containers in a consensus pod.
     *
     * @param total The number of pods expected.
     */
    private void checkSidecarContainers(final int total) {
        await().atMost(30, TimeUnit.SECONDS).untilAsserted(() -> {
            List<Pod> consensusPods = fetchConsensusPods();
            assertThat(consensusPods).hasSize(total);

            for (Pod p : consensusPods) {
                String accountId = p.getMetadata().getLabels().get(LABEL_ACCOUNT_ID);
                assertThat(accountId).isNotEmpty();
                assertThat(p.getSpec().getContainers().size())
                        .isGreaterThanOrEqualTo(5); // otel-collector is optional, so we are checking for 5 or more

                for (Container c : p.getSpec().getContainers()) {
                    Map<String, String> envMap = new HashMap<>();
                    for (EnvVar env : c.getEnv()) {
                        String value = env.getValue();
                        if (value == null || value.isEmpty()) {
                            value = env.getValueFrom().getFieldRef().getFieldPath();
                        }
                        envMap.put(env.getName(), value);
                    }

                    switch (c.getName()) {
                        case "record-stream-uploader":
                            assertThat(envMap.get("STREAM_EXTENSION"))
                                    .isEqualTo("%s.gz".formatted(RECORD_STREAM_EXTENSION));
                            assertThat(envMap.get("STREAM_FILE_EXTENSION")).isEqualTo(RECORD_STREAM_EXTENSION);
                            assertThat(envMap.get("STREAM_SIG_EXTENSION")).isEqualTo(RECORD_STREAM_SIG_EXTENSION);
                            assertThat(envMap.get("SIG_EXTENSION")).isEqualTo(RECORD_STREAM_SIG_EXTENSION);
                            assertThat(envMap.get("BUCKET_PATH"))
                                    .isEqualTo(RECORD_STREAM_BUCKET_PATH_TEMPLATE.formatted(accountId));
                            break;
                        case "events-stream-uploader":
                            assertThat(envMap.get("STREAM_EXTENSION")).isEqualTo(EVENT_STREAM_EXTENSION);
                            assertThat(envMap.get("STREAM_FILE_EXTENSION")).isEqualTo(EVENT_STREAM_EXTENSION);
                            assertThat(envMap.get("STREAM_SIG_EXTENSION")).isEqualTo(EVENT_STREAM_SIG_EXTENSION);
                            assertThat(envMap.get("SIG_EXTENSION")).isEqualTo(EVENT_STREAM_SIG_EXTENSION);
                            assertThat(envMap.get("BUCKET_PATH"))
                                    .isEqualTo(EVENT_STREAM_BUCKET_PATH_TEMPLATE.formatted(accountId));
                            break;
                        case "backup-uploader":
                            assertThat(envMap.get("RCLONE_S3_ACCESS_KEY_ID")).isNotEmpty();
                            assertThat(envMap.get("RCLONE_S3_ENDPOINT")).isNotEmpty();
                            assertThat(envMap.get("BACKUP_UPLOADER_BUCKET_1")).isNotEmpty();
                            break;
                        case "record-stream-sidecar-uploader":
                            assertThat(envMap.get("SIG_REQUIRE")).isEqualTo("false");
                            assertThat(envMap.get("SIG_PRIORITIZE")).isEqualTo("false");
                            assertThat(envMap.get("STREAM_EXTENSION"))
                                    .isEqualTo("%s.gz".formatted(RECORD_STREAM_EXTENSION));
                            assertThat(envMap.get("STREAM_FILE_EXTENSION")).isEqualTo(RECORD_STREAM_EXTENSION);
                            assertThat(envMap.get("STREAM_SIG_EXTENSION")).isEqualTo(RECORD_STREAM_SIG_EXTENSION);
                            assertThat(envMap.get("SIG_EXTENSION")).isEqualTo(RECORD_STREAM_SIG_EXTENSION);
                            assertThat(envMap.get("BUCKET_PATH"))
                                    .isEqualTo(RECORD_STREAM_BUCKET_PATH_TEMPLATE.formatted(accountId));
                            break;
                        case "open-telemetry-collector":
                            assertThat(envMap.get("OTEL_ENVIRONMENT")).isNotEmpty();
                            assertThat(envMap.get("OTEL_NODE_ID")).isNotEmpty();
                            assertThat(envMap.get("OTEL_METRICS_NODE_ID")).isNotEmpty();
                            assertThat(envMap.get("OTEL_ACCOUNT_ID")).isNotEmpty();
                            assertThat(envMap.get("OTEL_HOST_IP")).isNotEmpty();
                            assertThat(envMap.get("OTEL_ORBIT_NAME")).isNotEmpty();
                            assertThat(envMap.get("OTEL_APP_NAME")).isNotEmpty();
                            assertThat(envMap.get("OTEL_LOG_LEVEL")).isNotEmpty();
                            assertThat(envMap.get("OTEL_INSTANCE_TYPE")).isNotEmpty();
                            assertThat(envMap.get("OTEL_MEMORY_LIMITER_LIMIT_MIB"))
                                    .isNotEmpty();
                            assertThat(envMap.get("OTEL_MEMORY_LIMITER_SPIKE_LIMIT_MIB"))
                                    .isNotEmpty();

                            assertThat(c.getPorts().size()).isGreaterThan(0);
                            break;
                    }
                }
            }
        });
    }
}
