// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.test.capsuleconfigmap;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.test.fixtures.annotations.SoloTest;
import com.hedera.solo.operator.test.fixtures.snapshot.SnapshotFailureContext;
import com.hedera.solo.test.framework.AbstractOperatorTest;
import com.hedera.solo.test.networkoperation.NoOpOrbitReconciler;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.javaoperatorsdk.operator.junit.LocallyRunOperatorExtension;
import java.time.Duration;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.Test;

@SoloTest
public class CapsuleConfigMapTest extends AbstractOperatorTest {
    @Override
    protected LocallyRunOperatorExtension.Builder configure(final KubernetesClient client) {
        return builder(client)
                .withReconciler(new CapsuleConfigMapReconciler())
                .withReconciler(new NoOpOrbitReconciler());
    }

    @AfterEach
    void afterEach(final SnapshotFailureContext snapshot) {
        snapshot.client(operator().getKubernetesClient())
                .namespaces(operator().getNamespace())
                .namespacedResources();
    }

    @Test
    void test() {
        Orbit orbit = operator()
                .create(edit(new Orbit())
                        .with(metadata ->
                                metadata.editMetadata().withName(operator().getNamespace()))
                        .build());
        ConsensusCapsule capsule = operator()
                .create(edit(new ConsensusCapsule())
                        .with(metadata -> metadata.editMetadata()
                                .withNamespace(operator().getNamespace())
                                .withName("capsule"))
                        .with(spec -> spec.editSpec()
                                .withNodeId(0L)
                                .withAccountId("0.0.3")
                                .withOrbit(orbit.getMetadata().getName()))
                        .build());
        // verify default log4j2.xml
//        await().atMost(Duration.ofSeconds(30)).untilAsserted(() -> {
//            assertThat(operator().get(ConfigMap.class, "node0-cm").getData().get("log4j2.xml"))
//                    .isEqualTo(getSystemResourceAsString("import/hedera/mainnet/log4j2.xml"));
//        });

        // verify alternate log4j2.xml
        operator()
                .replace(edit(capsule)
                        .with(spec -> spec.editSpec().withLog4j2Xml("<?xml version=\"1.0\" encoding=\"UTF-8\"?>"))
                        .build());
        await().atMost(Duration.ofSeconds(30)).untilAsserted(() -> {
            assertThat(operator().get(ConfigMap.class, "node0-cm").getData().get("log4j2.xml"))
                    .isEqualTo("<?xml version=\"1.0\" encoding=\"UTF-8\"?>");
        });
    }
}
