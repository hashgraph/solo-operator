package com.hedera.solo.operator.reconciler.xp;

import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

import io.fabric8.kubernetes.api.model.ConfigMap;
import io.javaoperatorsdk.operator.junit.LocallyRunOperatorExtension;
import java.time.Duration;
import java.util.ArrayList;
import java.util.List;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.RegisterExtension;

public class StatusUpdaterTest {
    @RegisterExtension
    LocallyRunOperatorExtension operator =
            LocallyRunOperatorExtension.builder().withReconciler(new StatusUpdaterReconciler()).build();

    @Test
    void test() throws Exception {
        StatusUpdater statusUpdater = new StatusUpdater();
        List<StatusUpdater> list = new ArrayList<>();
        for (int i = 0; i < 100; i++) {
            statusUpdater.getMetadata().setName("example-" + i);
            list.add(operator.create(statusUpdater));
        }

        await().atMost(Duration.ofSeconds(30)).untilAsserted(() -> {
            ConfigMap cm = operator.get(ConfigMap.class, "status-updater");
            for (StatusUpdater updater : list) {
                assertThat(cm.hasOwnerReferenceFor(updater)).isTrue();
            }
        });

        list.forEach(operator::delete);

        await().atMost(Duration.ofSeconds(30)).untilAsserted(() -> {
            ConfigMap cm = operator.get(ConfigMap.class, "status-updater");
            assertThat(cm).isNull();
        });
    }
}
