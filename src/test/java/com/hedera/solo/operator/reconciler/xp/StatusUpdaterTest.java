package com.hedera.solo.operator.reconciler.xp;

import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

import com.hedera.solo.operator.model.v1alpha1.StatusUpdater;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.OwnerReference;
import io.javaoperatorsdk.operator.Operator;
import io.javaoperatorsdk.operator.junit.LocallyRunOperatorExtension;
import java.lang.invoke.MethodHandles;
import java.time.Duration;
import java.util.ArrayList;
import java.util.List;
import org.junit.jupiter.api.RepeatedTest;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.RegisterExtension;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

public class StatusUpdaterTest {
    private final Logger log = LoggerFactory.getLogger(MethodHandles.lookup().lookupClass());
    @RegisterExtension
    LocallyRunOperatorExtension operator =
            LocallyRunOperatorExtension.builder().withReconciler(new StatusUpdaterReconciler()).build();

    @RepeatedTest(1)
    void test() throws Exception {
        StatusUpdater statusUpdater = new StatusUpdater();
        List<StatusUpdater> list = new ArrayList<>();
        for (int i = 0; i < 10; i++) {
            statusUpdater.getMetadata().setName("example-" + i);
            list.add(operator.create(statusUpdater));
        }

        await().atMost(Duration.ofSeconds(90)).untilAsserted(() -> {
            ConfigMap cm = operator.get(ConfigMap.class, "status-updater");
            ConfigMap cm2 = operator.get(ConfigMap.class, "status-updater-no-ssa");
            assertThat(cm).isNotNull();
            assertThat(cm2).isNotNull();
            for (StatusUpdater updater : list) {
                    assertThat(cm.hasOwnerReferenceFor(updater)).isTrue();
                    assertThat(cm2.hasOwnerReferenceFor(updater)).isTrue();
            }
        });

        operator.delete(operator.get(ConfigMap.class, "status-updater"));
        operator.delete(operator.get(ConfigMap.class, "status-updater-no-ssa"));

        await().atMost(Duration.ofSeconds(90)).untilAsserted(() -> {
            ConfigMap cm = operator.get(ConfigMap.class, "status-updater");
            ConfigMap cm2 = operator.get(ConfigMap.class, "status-updater-no-ssa");
            assertThat(cm).isNotNull();
            assertThat(cm2).isNotNull();
            for (StatusUpdater updater : list) {
                    assertThat(cm.hasOwnerReferenceFor(updater)).isTrue();
                    assertThat(cm2.hasOwnerReferenceFor(updater)).isTrue();
            }
        });

        list.forEach(operator::delete);

        await().atMost(Duration.ofSeconds(30)).untilAsserted(() -> {
            assertThat(operator.get(ConfigMap.class, "status-updater")).isNull();
            assertThat(operator.get(ConfigMap.class, "status-updater-no-ssa")).isNull();
        });
    }
}
