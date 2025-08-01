// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.capsule;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.operator.library.data.AsciiArmor.base64Decode;
import static com.hedera.solo.operator.library.data.AsciiArmor.base64Encode;
import static com.hedera.solo.operator.reconciler.common.Constants.KEY_TYPE_GRPC_TLS;
import static com.hedera.solo.operator.reconciler.common.Constants.LABEL_SELECTOR_HAPROXY_APP;
import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

import com.hedera.solo.operator.model.v1alpha1.HAProxyCapsule;
import com.hedera.solo.operator.model.v1alpha1.HAProxyCapsuleBuilder;
import com.hedera.solo.operator.model.v1alpha1.HAProxyCapsuleSpec;
import com.hedera.solo.operator.model.v1alpha1.HAProxyCapsuleStatus;
import com.hedera.solo.operator.reconciler.capsule.haproxy.resources.HAProxyService;
import com.hedera.solo.operator.reconciler.common.Metadata;
import com.hedera.solo.operator.test.utils.TestRecords;
import io.fabric8.kubernetes.api.model.EnvVar;
import io.fabric8.kubernetes.api.model.Pod;
import io.fabric8.kubernetes.api.model.Secret;
import io.fabric8.kubernetes.api.model.apps.Deployment;
import io.fabric8.kubernetes.client.CustomResource;
import io.quarkus.test.junit.QuarkusTest;
import java.time.Duration;
import java.util.Arrays;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Set;
import java.util.concurrent.TimeUnit;
import java.util.function.Supplier;
import java.util.stream.Collectors;
import org.junit.jupiter.api.Test;

/** This class contains tests for the HAProxyCapsule custom resource. */
@QuarkusTest
public class HAProxyCapsuleTest extends ConsensusCapsuleTestAbstract {
    /** Cleans up any remaining resources after test execution. */
    @Override
    void cleanup() {
        kubernetesClient.resources(HAProxyCapsule.class).inNamespace(namespace).delete();
        await().atMost(Duration.ofSeconds(30)).untilAsserted(() -> {
            assertThat(kubernetesClient
                            .resources(HAProxyCapsule.class)
                            .inNamespace(namespace)
                            .list()
                            .getItems())
                    .isEmpty();
        });
        super.cleanup();
    }

    /** Tests the deployment of a single HAProxy capsule. */
    @Override
    @Test
    void testSingleNodeDeployment() throws Exception {
        super.testSingleNodeDeployment();
        Long[] consensusNodeIds = createHAProxyCapsule();
        simulateCertificateUpdate(consensusNodeIds);
    }

    /** Tests the deployment of multiple HAProxy capsules. */
    @Override
    @Test
    void testMultiNodeDeployment() throws Exception {
        super.testMultiNodeDeployment();
        Long[] consensusNodeIds = createHAProxyCapsule();
        simulateCertificateUpdate(consensusNodeIds);
    }

    @Override
    protected void setupSecrets(final List<TestRecords.NodeInfo> nodes) {
        super.setupSecrets(nodes);
        for (final TestRecords.NodeInfo node : nodes) {
            final String secretName = "%s-%s-keys".formatted(node.name(), KEY_TYPE_GRPC_TLS);
            Secret secret = kubernetesClient
                    .secrets()
                    .inNamespace(namespace)
                    .withName(secretName)
                    .get();
            kubernetesClient
                    .secrets()
                    .inNamespace(namespace)
                    .resource(edit(new Secret())
                            .with(metadata -> metadata.editMetadata()
                                    .withName("haproxy-secret-" + node.id())
                                    .withNamespace(namespace))
                            .with(data -> data.addToData(
                                    "hedera.pem",
                                    base64Encode(base64Decode(secret.getData().get("hedera-" + node.name() + ".key"))
                                                    .trim()
                                            + "\n"
                                            + base64Decode(secret.getData().get("hedera-" + node.name() + ".crt"))
                                                    .trim()
                                            + "\n")))
                            .build())
                    .create();
        }
        await().atMost(Duration.ofSeconds(30)).untilAsserted(() -> {
            assertThat(kubernetesClient.secrets().inNamespace(namespace).list().getItems().stream()
                            .filter(s -> s.getMetadata().getName().startsWith("haproxy-secret-"))
                            .count())
                    .isEqualTo(nodes.size());
        });
    }

    /**
     * Creates HAProxy capsules based on the specified consensus node IDs.
     *
     * @return Array of consensus node IDs for which HAProxy capsules are created
     * @throws Exception if an error occurs while creating the capsules.
     */
    Long[] createHAProxyCapsule() throws Exception {
        Long[] consensusNodeIds = consensusNodeIds();
        HAProxyCapsuleSpec spec = new HAProxyCapsuleSpec();
        spec.setEnv(null);
        for (Long nodeId : consensusNodeIds) {
            HAProxyCapsule haProxyCapsule = new HAProxyCapsuleBuilder()
                    .editMetadata()
                    .withNamespace(namespace)
                    .withName(Metadata.prepareNodeName(nodeId))
                    .endMetadata()
                    .withNewSpecLike(spec)
                    .withSecretName("haproxy-secret-" + nodeId)
                    .withEnv(
                            new EnvVar("HAPROXY_TARGET_IP", Metadata.prepareNodeName(nodeId) + "-svc", null),
                            new EnvVar("HAPROXY_USER", "user", null),
                            new EnvVar("HAPROXY_AUTH", "auth", null))
                    .endSpec()
                    .build();
            kubernetesClient.resource(haProxyCapsule).create();
        }
        waitForCapsules(consensusNodeIds);
        return consensusNodeIds;
    }

    /**
     * Waits for the HAProxy capsules to be deployed and verifies their status.
     *
     * @param consensusNodeIds Array of consensus node IDs for which the capsules are expected to be deployed.
     * @throws Exception if an error occurs while waiting for the capsules.
     */
    void waitForCapsules(final Long[] consensusNodeIds) throws Exception {
        await().atMost(150, TimeUnit.SECONDS).untilAsserted(() -> {
            List<HAProxyCapsule> proxies = kubernetesClient
                    .resources(HAProxyCapsule.class)
                    .inNamespace(namespace)
                    .list()
                    .getItems();
            assertThat(proxies).hasSize(consensusNodeIds.length);
            assertThat(proxies.stream()).map(CustomResource::getStatus).allMatch(Objects::nonNull);
            assertThat(proxies.stream())
                    .map(CustomResource::getStatus)
                    .map(HAProxyCapsuleStatus::getPhase)
                    .allMatch(HAProxyCapsuleStatus.Phase.Running::equals);
        });

        // test with client
        checkAddressBook((f, n, s) -> f.haproxyContextCreator(services(), n, s));
    }

    /**
     * Returns an array of consensus node IDs for which HAProxy capsules are created.
     *
     * @return Array of consensus node IDs
     */
    Map<String, String> services() {
        return Arrays.stream(consensusNodeIds())
                .collect(Collectors.toMap(
                        Metadata::prepareNodeName,
                        nodeId -> Metadata.prepareNodeName(nodeId) + HAProxyService.NAME_SUFFIX));
    }

    /**
     * Simulates an update to the consensus secret's hedera.crt field and verifies the HAProxy deployments are updated
     * accordingly.
     *
     * @param consensusNodeIds Array of consensus node IDs for which the certificate update is simulated.
     * @throws Exception if an error occurs during the update simulation.
     */
    void simulateCertificateUpdate(final Long[] consensusNodeIds) throws Exception {
        Supplier<Map<String, String>> checksums = () -> kubernetesClient
                .resources(Deployment.class)
                .inNamespace(namespace)
                .withLabel(LABEL_SELECTOR_HAPROXY_APP)
                .list()
                .getItems()
                .stream()
                .collect(Collectors.toMap(deployment -> deployment.getMetadata().getName(), deployment -> deployment
                        .getSpec()
                        .getTemplate()
                        .getMetadata()
                        .getAnnotations()
                        .get("checksum/secret")));

        // initial observed generations
        Map<String, String> initialChecksums = checksums.get();

        // modify hedera.crt in consensus secret
        String fieldName = "hedera.pem";
        for (Long nodeId : consensusNodeIds) {
            Secret secret = kubernetesClient
                    .secrets()
                    .inNamespace(namespace)
                    .withName("haproxy-secret-" + nodeId)
                    .get();
            kubernetesClient.resource(secret).edit(s -> s.edit()
                    .addToData(fieldName + ".orig", s.getData().get(fieldName))
                    .addToData(fieldName, "AA==")
                    .build());
        }

        // verify haproxy deployments are updated
        await().atMost(30, TimeUnit.SECONDS).untilAsserted(() -> {
            Map<String, String> currentChecksums = checksums.get();
            assertThat(currentChecksums.entrySet())
                    .noneMatch(e -> Objects.equals(initialChecksums.get(e.getKey()), e.getValue()));
        });

        // restore hedera.crt consensus secret
        for (Long nodeId : consensusNodeIds) {
            Secret secret = kubernetesClient
                    .secrets()
                    .inNamespace(namespace)
                    .withName("haproxy-secret-" + nodeId)
                    .get();
            kubernetesClient.resource(secret).edit(s -> s.edit()
                    .addToData(fieldName, s.getData().get(fieldName + ".orig"))
                    .removeFromData(fieldName + ".orig")
                    .build());
        }

        // verify haproxy deployments are restored
        await().atMost(60, TimeUnit.SECONDS).untilAsserted(() -> {
            Map<String, String> currentChecksums = checksums.get();
            assertThat(currentChecksums.entrySet())
                    .allMatch(e -> Objects.equals(initialChecksums.get(e.getKey()), e.getValue()));
            Set<String> podChecksums = kubernetesClient
                    .resources(Pod.class)
                    .inNamespace(namespace)
                    .withLabel("app.kubernetes.io/part-of", "haproxy")
                    .list()
                    .getItems()
                    .stream()
                    .map(pod -> pod.getMetadata().getAnnotations().get("checksum/secret"))
                    .collect(Collectors.toSet());
            assertThat(podChecksums).isEqualTo(new HashSet<>(initialChecksums.values()));
        });

        waitForCapsules(consensusNodeIds);
    }
}
