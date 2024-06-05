package com.hedera.solo.operator.reconciler.xp;

import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

import com.hedera.solo.operator.v1alpha1.StatusUpdater;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.javaoperatorsdk.operator.junit.LocallyRunOperatorExtension;
import java.time.Duration;
import java.util.ArrayList;
import java.util.List;
import org.junit.jupiter.api.RepeatedTest;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.RegisterExtension;

public class StatusUpdaterTest {
    @RegisterExtension
    LocallyRunOperatorExtension operator =
            LocallyRunOperatorExtension.builder().withReconciler(new StatusUpdaterReconciler()).build();

    @RepeatedTest(30)
    void test() throws Exception {
        StatusUpdater statusUpdater = new StatusUpdater();
        List<StatusUpdater> list = new ArrayList<>();
        for (int i = 0; i < 1000; i++) {
            statusUpdater.getMetadata().setName("example-" + i);
            list.add(operator.create(statusUpdater));
//            Thread.sleep(100);
        }

        await().atMost(Duration.ofSeconds(30)).untilAsserted(() -> {
            ConfigMap cm = operator.get(ConfigMap.class, "status-updater");
            ConfigMap cm2 = operator.get(ConfigMap.class, "status-updater-no-ssa");
            assertThat(cm).isNotNull();
            assertThat(cm2).isNotNull();
            for (StatusUpdater updater : list) {
                assertThat(cm2.hasOwnerReferenceFor(updater)).isTrue();
//                assertThat(cm.hasOwnerReferenceFor(updater)).isTrue();
            }
            for (StatusUpdater updater : list) {
//                assertThat(cm2.hasOwnerReferenceFor(updater)).isTrue();
//                assertThat(cm.hasOwnerReferenceFor(updater)).isTrue();
            }
        });

        list.forEach(operator::delete);

        await().atMost(Duration.ofSeconds(30)).untilAsserted(() -> {
            assertThat(operator.get(ConfigMap.class, "status-updater")).isNull();
//            assertThat(operator.get(ConfigMap.class, "status-updater-no-ssa")).isNull();
        });
    }
}
