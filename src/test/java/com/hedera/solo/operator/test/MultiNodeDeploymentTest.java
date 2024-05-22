package com.hedera.solo.operator.test;

import static com.hedera.solo.operator.reconciler.common.Metadata.validName;
import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;
import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsuleBuilder;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsuleStatus;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitialization;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitializationBuilder;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitializationSpec;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitializationStatus;
import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.model.v1alpha1.OrbitBuilder;
import com.hedera.solo.operator.reconciler.capsule.consensus.resources.CapsuleDeployment;
import com.hedera.solo.operator.test.fixtures.Environment;
import com.hedera.solo.operator.test.fixtures.Resources;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.HasMetadata;
import io.fabric8.kubernetes.api.model.Secret;
import io.fabric8.kubernetes.api.model.Service;
import io.fabric8.kubernetes.api.model.VolumeDevice;
import io.fabric8.kubernetes.api.model.VolumeDeviceBuilder;
import io.fabric8.kubernetes.api.model.VolumeMount;
import io.fabric8.kubernetes.api.model.VolumeMountBuilder;
import io.fabric8.kubernetes.api.model.apps.Deployment;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.javaoperatorsdk.operator.Operator;
import io.quarkus.test.junit.QuarkusTest;
import jakarta.inject.Inject;
import java.lang.invoke.MethodHandles;
import java.time.Duration;
import java.util.ArrayList;
import java.util.HashSet;
import java.util.List;
import java.util.Set;
import java.util.concurrent.TimeUnit;
import java.util.function.Function;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

@QuarkusTest
public class MultiNodeDeploymentTest {
    private static final Logger logger = LoggerFactory.getLogger(MethodHandles.lookup().lookupClass());
    private static final String NAMESPACE = MethodHandles.lookup().lookupClass().getSimpleName().toLowerCase();
    private static final String TEST_ORBIT_NAME = NAMESPACE;
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
        client.resources(ConsensusCapsule.class).inNamespace(NAMESPACE).delete();
        Resources.removeByName(client, Orbit.class, TEST_ORBIT_NAME);
        await().atMost(Duration.ofSeconds(30))
                .until(() -> client.namespaces().withName(TEST_ORBIT_NAME).get() == null);
    }

    @Test
    void testDeploy() throws Exception {
        Deployment deployment = loadYaml(Deployment.class, CapsuleDeployment.class, "deployment.yaml");
        Secret publicKeys = loadYaml(Secret.class, getClass(), "public-keys.yaml");
        Secret privateKeys = loadYaml(Secret.class, getClass(), "node0-keys.yaml");

        String validName = "node0";
        String metadataName = "node0";

        List<VolumeMount> volumeMounts = new ArrayList<>();
        Set<String> names = new HashSet<>(privateKeys.getData().keySet());
        names.removeAll(Set.of("hedera.crt", "hedera.key"));
        for (String name : names) {
            volumeMounts.add(new VolumeMountBuilder()
                    .withName("private-keys")
                    .withMountPath("/opt/hgcapp/services-hedera/HapiApp2.0/data/keys/" + name)
                    .withSubPath(name)
                    .build());
        }
        for (String name : publicKeys.getData().keySet()) {
            volumeMounts.add(new VolumeMountBuilder()
                    .withName("public-keys")
                    .withMountPath("/opt/hgcapp/services-hedera/HapiApp2.0/data/keys/" + name)
                    .withSubPath(name)
                    .build());
        }
//
//        deployment.edit().editSpec().editTemplate().editSpec().editMatchingContainer(p -> "consensus-node".equals(p.getName())).addAllToVolumeDevices();

/*
# the following needs to be added dynamically
#            - name: private-keys
#              mountPath: /opt/hgcapp/services-hedera/HapiApp2.0/data/keys
#              subPath: a-private-node0.pem
#            - name: private-keys
#              mountPath: /opt/hgcapp/services-hedera/HapiApp2.0/data/keys
#              subPath: s-private-node0.pem

# for each file in the public-keys configmap:
#            - name: public-keys
#              mountPath: /opt/hgcapp/services-hedera/HapiApp2.0/data/keys
#              subPath: ${file}
* */
        deployment = deployment.edit()
                .editMetadata()
                .withName(validName)
                .withNamespace("metadata.getNamespace()")
                .endMetadata()
                .editSpec()
                .withReplicas(42)
                .editTemplate()
                .editSpec()
                .editMatchingVolume(v -> "private-keys".equals(v.getName()))
                .editSecret()
                .withSecretName(validName(metadataName, "-keys"))
                .endSecret()
                .endVolume()
                .editMatchingContainer(c -> "consensus-node".equals(c.getName()))
                .addAllToVolumeMounts(volumeMounts)
                .endContainer()
                .endSpec()
                .endTemplate()
                .endSpec()
                .build();

        logger.info("Deployment {}", new ObjectMapper().writerWithDefaultPrettyPrinter().writeValueAsString(deployment));
        int i = 1;
    }

    record Node(String name, int weight, String internal, String external) {}

    ConfigMap config(ConfigMap configMap, Node... nodes) {
        String chainId = configMap.getData().get("bootstrap.properties").lines().filter(s -> s.startsWith("contracts.chainId=")).findFirst().get().substring(18);
        int nodeId = 0;
        StringBuilder sb = new StringBuilder();
        sb.append("swirld, ").append(chainId).append("\n");
        sb.append("app, HederaNode.jar\n");
        for (Node node : nodes) {
            sb.append("address, %d, %s, %s, 1, %s, 50111, %s, 50111, 0.0.%d\n"
                    .formatted(nodeId, node.name, node.name, node.internal, node.external, nodeId + 3));
            nodeId++;
        }
        sb.append("nextNodeId, ").append(nodeId);
        return configMap.edit().addToData("config.txt", sb.toString()).build();
    }

//    @Test
//    void testClusterConfig() throws Exception {
//        ConfigMap configMap = loadYaml(ConfigMap.class, getClass(), "cluster-config.yaml");
//        String namespace = TEST_ORBIT_NAME;
//        String chainId = configMap.getData().get("bootstrap.properties").lines().filter(s -> s.startsWith("contracts.chainId=")).findFirst().get().substring(18);
//        Function<String, String> internalName = s -> s + "-svc." + namespace + "." + "svc.cluster.local";
//        Function<String, String> externalName = internalName;//s -> s + "-svc." + namespace + "." + "svc.cluster.local";
//
//        String config = config(chainId,
//                new Node("node0", 1, internalName.apply("node0"), externalName.apply("node0")),
//                new Node("node1", 1, internalName.apply("node1"), externalName.apply("node1")),
//                new Node("node2", 1, internalName.apply("node2"), externalName.apply("node2")));
//
//        configMap = configMap.edit().addToData("config.txt", config).build();
//
//        logger.info("configMap {}", new ObjectMapper().writerWithDefaultPrettyPrinter().writeValueAsString(configMap));
//        int i = 1;
//    }
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

        // populate config and keys
        Function<String, String> internalName = s -> s + "-int." + NAMESPACE + "." + "svc.cluster.local";
        Function<String, String> externalName = s -> s + "-ext." + NAMESPACE + "." + "svc.cluster.local";
        client.resource(config(loadYaml(ConfigMap.class, getClass(), "cluster-config.yaml").edit()
                        .editMetadata().withNamespace(NAMESPACE).endMetadata().build(),
                new Node("node0", 1, internalName.apply("node0"), externalName.apply("node0")),
                new Node("node1", 1, internalName.apply("node1"), externalName.apply("node1")),
                new Node("node2", 1, internalName.apply("node2"), externalName.apply("node2"))))
                .create();
        client.resource(loadYaml(ConfigMap.class, getClass(), "public-keys.yaml").edit()
                        .editMetadata().withNamespace(NAMESPACE).endMetadata().build()).create();

        // create consensus capsule
        ConsensusCapsule node0 = createConsensusCapsule("node0");
        ConsensusCapsule node1 = createConsensusCapsule("node1");
        ConsensusCapsule node2 = createConsensusCapsule("node2");

        waitForConsensusCapsules(node0, node1, node2);

        client.resource(loadYaml(Secret.class, getClass(), "node0-keys.yaml")).inNamespace(NAMESPACE).patch();
        client.resource(loadYaml(Secret.class, getClass(), "node1-keys.yaml")).inNamespace(NAMESPACE).patch();
        client.resource(loadYaml(Secret.class, getClass(), "node2-keys.yaml")).inNamespace(NAMESPACE).patch();

        // create network initialization
        final NetworkInitialization networkInitialization = new NetworkInitializationBuilder()
                .withNewMetadata()
                .withName("network-initialization")
                .withNamespace(NAMESPACE)
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
                .withNamespace(NAMESPACE)
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
//                ConfigMap configMap = client.configMaps().inNamespace(TEST_ORBIT_NAME).withName(name + "-cm").get();
//                assertThat(configMap).isNotNull();
                Secret secret = client.secrets().inNamespace(NAMESPACE).withName(name + "-keys").get();
                assertThat(secret).isNotNull();
                Service service = client.services().inNamespace(NAMESPACE).withName(name + "-int").get();
                assertThat(service).isNotNull();
                service = client.services().inNamespace(NAMESPACE).withName(name + "-ext").get();
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

                Deployment deployment = client.resources(Deployment.class).inNamespace(NAMESPACE).withName(name + "-deploy").get();
                assertThat(deployment).isNotNull();
                assertThat(deployment.getStatus()).isNotNull();
                assertThat(deployment.getStatus().getAvailableReplicas()).isEqualTo(1);

                assertThat(capsule.getStatus().getPhase()).isEqualTo(ConsensusCapsuleStatus.Phase.Running);
                assertThat(netInit.getStatus().getPhase()).isEqualTo(NetworkInitializationStatus.Phase.Running);
            }
        });
    }
}
