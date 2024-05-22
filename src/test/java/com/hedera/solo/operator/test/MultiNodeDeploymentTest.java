package com.hedera.solo.operator.test;

import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;
import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsuleBuilder;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsuleStatus;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitialization;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitializationBuilder;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitializationSpec;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitializationStatus;
import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.model.v1alpha1.OrbitBuilder;
import com.hedera.solo.operator.test.fixtures.Environment;
import com.hedera.solo.operator.test.fixtures.Resources;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.Secret;
import io.fabric8.kubernetes.api.model.Service;
import io.fabric8.kubernetes.api.model.apps.Deployment;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.javaoperatorsdk.operator.Operator;
import io.quarkus.test.junit.QuarkusTest;
import jakarta.inject.Inject;
import java.lang.invoke.MethodHandles;
import java.time.Duration;
import java.util.concurrent.TimeUnit;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

@QuarkusTest
public class MultiNodeDeploymentTest {
    private static final Logger logger = LoggerFactory.getLogger(MethodHandles.lookup().lookupClass());
    private static final String TEST_ORBIT_NAME = MethodHandles.lookup().lookupClass().getSimpleName().toLowerCase();
    @Inject
    @SuppressWarnings("CdiInjectionPointsInspection")
    protected KubernetesClient client;

    @Inject
    protected Operator operator;

    @Inject
    protected org.jboss.logging.Logger log;

    @BeforeEach
    void setup() {
        if (!Environment.isDevModeActive()) {
            operator.start();
        }
        cleanup();
    }

    @AfterEach
    void cleanup() {
        client.resources(ConsensusCapsule.class).inNamespace(TEST_ORBIT_NAME).delete();
        Resources.removeByName(client, Orbit.class, TEST_ORBIT_NAME);
        await().atMost(Duration.ofSeconds(30))
                .until(() -> client.namespaces().withName(TEST_ORBIT_NAME).get() == null);
    }

    @Test
    void testMultiNodeDeployment() {
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
        ConsensusCapsule node0 = createConsensusCapsule("node0");
        ConsensusCapsule node1 = createConsensusCapsule("node1");
        ConsensusCapsule node2 = createConsensusCapsule("node2");

        waitForConsensusCapsules(node0, node1, node2);

        // create network initialization
        final NetworkInitialization networkInitialization = new NetworkInitializationBuilder()
                .withNewMetadata()
                .withName("network-initialization")
                .withNamespace(TEST_ORBIT_NAME)
                .endMetadata()
                .withNewSpec().withMode(NetworkInitializationSpec.Mode.FirstInitialization).endSpec()
                .withNewStatus().endStatus()
                .build();
        client.resource(networkInitialization).create();

        waitForDeployments(networkInitialization, node0, node1, node2);
    }

    ConsensusCapsule createConsensusCapsule(String name) {
        final ConsensusCapsule consensusCapsule = new ConsensusCapsuleBuilder()
                .withNewMetadata()
                .withName(name)
                .withNamespace(TEST_ORBIT_NAME)
                .endMetadata()
                .withNewSpec().endSpec()
                .build();
        return client.resource(consensusCapsule).create();
    }

    void waitForConsensusCapsules(ConsensusCapsule... consensusCapsules) {
        for (ConsensusCapsule consensusCapsule : consensusCapsules) {
            String name = consensusCapsule.getMetadata().getName();
            await().atMost(30, TimeUnit.SECONDS).untilAsserted(() -> {
                ConsensusCapsule capsule = client.resource(consensusCapsule).get();
                assertThat(capsule).isNotNull();
                ConfigMap configMap = client.configMaps().inNamespace(TEST_ORBIT_NAME).withName(name + "-cm").get();
                assertThat(configMap).isNotNull();
                Secret secret = client.secrets().inNamespace(TEST_ORBIT_NAME).withName(name + "-secret").get();
                assertThat(secret).isNotNull();
                Service service = client.services().inNamespace(TEST_ORBIT_NAME).withName(name + "-svc").get();
                assertThat(service).isNotNull();
                assertThat(capsule.getSpec().getEligibleForDeployment()).isFalse();
            });
        }
    }

    void waitForDeployments(NetworkInitialization networkInitialization, ConsensusCapsule... consensusCapsules) {
        await().atMost(90, TimeUnit.SECONDS).untilAsserted(() -> {
            NetworkInitialization netInit = client.resource(networkInitialization).get();
            assertThat(netInit).isNotNull();
            for (ConsensusCapsule consensusCapsule : consensusCapsules) {
                String name = consensusCapsule.getMetadata().getName();
                ConsensusCapsule capsule = client.resource(consensusCapsule).get();
                assertThat(capsule).isNotNull();
                assertThat(capsule.getSpec().getEligibleForDeployment()).isTrue();

                Deployment deployment = client.resources(Deployment.class).inNamespace(TEST_ORBIT_NAME).withName(name + "-deploy").get();
                assertThat(deployment).isNotNull();
                assertThat(deployment.getStatus()).isNotNull();
                assertThat(deployment.getStatus().getAvailableReplicas()).isEqualTo(1);

                assertThat(capsule.getStatus().getPhase()).isEqualTo(ConsensusCapsuleStatus.Phase.Running);
                assertThat(netInit.getStatus().getPhase()).isEqualTo(NetworkInitializationStatus.Phase.Running);
            }
        });
    }
}
