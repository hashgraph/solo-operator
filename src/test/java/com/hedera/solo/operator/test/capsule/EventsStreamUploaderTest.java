// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.capsule;

import static com.hedera.solo.operator.reconciler.common.Constants.EVENT_STREAM_BUCKET_PATH_TEMPLATE;
import static com.hedera.solo.operator.reconciler.common.Constants.EVENT_STREAM_EXTENSION;
import static com.hedera.solo.operator.reconciler.common.Constants.EVENT_STREAM_SIG_EXTENSION;
import static com.hedera.solo.operator.reconciler.common.Constants.LABEL_ACCOUNT_ID;
import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;
import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

import com.hedera.solo.operator.model.commonspec.StreamUploader;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.podproperties.Containers;
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
public class EventsStreamUploaderTest extends ConsensusCapsuleTestAbstract {
    public static final String EVENT_STREAM_TEMPLATE = "containers/events-stream-uploader.yaml";

    @Override
    @Test
    void testSingleNodeDeployment() throws Exception {
        super.testSingleNodeDeployment();
        this.checksEventStreamUploader(1);
    }

    @Override
    @Test
    void testMultiNodeDeployment() throws Exception {
        super.testMultiNodeDeployment();
        this.checksEventStreamUploader(3);
    }

    @Override
    protected Containers prepareConsensusPodContainers() {
        StreamUploader uploader = loadYaml(StreamUploader.class, getClass(), EVENT_STREAM_TEMPLATE);
        return getContainersBuilder().withEventsStreamUploader(uploader).build();
    }

    private void checksEventStreamUploader(final int total) {
        await().atMost(30, TimeUnit.SECONDS).untilAsserted(() -> {
            List<Pod> consensusPods = fetchConsensusPods();
            assertThat(consensusPods).hasSize(total);

            for (Pod p : consensusPods) {
                String accountId = p.getMetadata().getLabels().get(LABEL_ACCOUNT_ID);
                assertThat(accountId).isNotEmpty();

                for (Container c : p.getSpec().getContainers()) {
                    if (c.getName().equals("events-stream-uploader")) {
                        Map<String, String> envMap = new HashMap<>();
                        for (EnvVar env : c.getEnv()) {
                            envMap.put(env.getName(), env.getValue());
                        }

                        assertThat(envMap.get("STREAM_EXTENSION")).isEqualTo(EVENT_STREAM_EXTENSION);
                        assertThat(envMap.get("STREAM_FILE_EXTENSION")).isEqualTo(EVENT_STREAM_EXTENSION);
                        assertThat(envMap.get("STREAM_SIG_EXTENSION")).isEqualTo(EVENT_STREAM_SIG_EXTENSION);
                        assertThat(envMap.get("SIG_EXTENSION")).isEqualTo(EVENT_STREAM_SIG_EXTENSION);
                        assertThat(envMap.get("BUCKET_PATH"))
                                .isEqualTo(EVENT_STREAM_BUCKET_PATH_TEMPLATE.formatted(accountId));

                        break;
                    }
                }
            }
        });
    }
}
