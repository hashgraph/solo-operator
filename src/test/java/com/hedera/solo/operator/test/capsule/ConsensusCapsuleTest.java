package com.hedera.solo.operator.test.capsule;

import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsuleBuilder;
import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.model.v1alpha1.OrbitBuilder;
import com.hedera.solo.operator.test.fixtures.Environment;
import com.hedera.solo.operator.test.fixtures.Resources;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.Secret;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.javaoperatorsdk.operator.Operator;
import io.quarkus.test.junit.QuarkusTest;
import jakarta.inject.Inject;
import java.lang.invoke.MethodHandles;
import java.time.Duration;
import java.util.concurrent.TimeUnit;
import org.jboss.logging.Logger;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

@QuarkusTest
public class ConsensusCapsuleTest {
    private static final String TEST_ORBIT_NAME = MethodHandles.lookup().lookupClass().getSimpleName().toLowerCase();
    @Inject
    @SuppressWarnings("CdiInjectionPointsInspection")
    protected KubernetesClient client;

    @Inject
    protected Operator operator;

    @Inject
    protected Logger log;

    @BeforeEach
    void setup() throws InterruptedException {
        if (!Environment.isDevModeActive()) {
            operator.start();
            operator.installShutdownHook(Duration.ofSeconds(30));
        }
        cleanup();
    }

    @AfterEach
    void cleanup() throws InterruptedException {
        client.resources(ConsensusCapsule.class).inNamespace(TEST_ORBIT_NAME).delete();
        Resources.removeByName(client, Orbit.class, TEST_ORBIT_NAME);
        await().atMost(Duration.ofSeconds(30))
                .until(() -> client.namespaces().withName(TEST_ORBIT_NAME).get() == null);
    }

    @Test
    void testConsensusCapsule() {
        // create orbit
        final Orbit orbit = new OrbitBuilder()
                .withNewMetadata().withName(TEST_ORBIT_NAME).endMetadata()
                .withNewSpec().endSpec()
                .build();
        client.resource(orbit).create();

        await().atMost(30, TimeUnit.SECONDS).untilAsserted(() -> {
            assertThat(client.resource(orbit).get()).isNotNull();
        });

        // create consensus capsule
        final String consensusCapsuleName = "alpha";
        final ConsensusCapsule consensusCapsule = new ConsensusCapsuleBuilder()
                .withNewMetadata()
                .withName(consensusCapsuleName)
                .withNamespace(TEST_ORBIT_NAME)
                .endMetadata()
                .build();
        client.resource(consensusCapsule).create();

        await().atMost(30, TimeUnit.SECONDS).untilAsserted(() -> {
            ConsensusCapsule capsule = client.resource(consensusCapsule).get();
            assertThat(capsule).isNotNull();
            ConfigMap configMap = client.configMaps().inNamespace(TEST_ORBIT_NAME).withName(consensusCapsuleName + "-cm").get();
            assertThat(configMap).isNotNull();
            Secret secret = client.secrets().inNamespace(TEST_ORBIT_NAME).withName(consensusCapsuleName + "-secret").get();
            assertThat(secret).isNotNull();
        });
    }
}
