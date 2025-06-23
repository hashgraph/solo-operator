// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.capsule;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;
import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.test.fixtures.hederaclient.HederaSdkClientContextFactory;
import com.hedera.solo.operator.test.fixtures.hederaclient.HederaSdkClientFactory;
import com.hedera.solo.operator.test.utils.TestRecords;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.Pod;
import io.fabric8.kubernetes.api.model.Volume;
import io.fabric8.kubernetes.api.model.VolumeBuilder;
import io.fabric8.kubernetes.api.model.VolumeMount;
import io.fabric8.kubernetes.api.model.VolumeMountBuilder;
import io.quarkus.test.junit.QuarkusTest;
import java.time.Duration;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Disabled;
import org.junit.jupiter.api.Test;

/** This class contains tests for the SecretHostpath functionality in ConsensusCapsule. */
@QuarkusTest
public class SecretHostpathTest extends ConsensusCapsuleTestAbstract {
    boolean setup;
    List<ConfigMap> configMaps = new ArrayList<>();
    Map<String, byte[]> certificates = new HashMap<>();
    Map<String, String> pubKeys = new HashMap<>();
    List<Volume> volumes = new ArrayList<>();
    List<VolumeMount> volumeMounts = new ArrayList<>();

    /** Sets up the test environment. */
    @Override
    @BeforeEach
    void setup() {
        setup = true;
        try {
            super.setup();
            ConfigMap configMap = loadYaml(ConfigMap.class, getClass(), "secrethostpath/config.yaml");
            createConfigMap(configMap);
            addVolumes("src-config", "dst-config", configMap);
            for (int i = 1; i <= 3; ++i) {
                String nodeName = "node" + i;
                configMap = loadYaml(ConfigMap.class, getClass(), "secrethostpath/" + nodeName + "-src.yaml");
                certificates.put(nodeName, configMap.getData().get("hedera.crt").getBytes());
                pubKeys.put(nodeName, configMap.getData().get("s-public-" + nodeName + ".pem"));
                createConfigMap(configMap);
                addVolumes(nodeName + "-src", nodeName + "-dst", configMap);
            }
            runInPod(
                    volumes,
                    volumeMounts,
                    "find -L /mnt/src -maxdepth 2 -mindepth 2 -type f -print | sed 's,\\(/mnt/src/\\(.*\\)\\),cp \\1 /mnt/dst/\\2,' | sh");
        } finally {
            setup = false;
        }
    }

    /** Cleans up the test environment. */
    @Override
    void cleanup() {
        if (!setup) {
            runInPod(volumes, volumeMounts, "find /mnt/dst -type f -exec rm {} \\;");
            configMaps.forEach(configMap -> kubernetesClient.resource(configMap).delete());
        }
        super.cleanup();
    }

    /** Tests the deployment of a single node. */
    @Override
    @Disabled("not required")
    void testSingleNodeDeployment() throws Exception {
        super.testSingleNodeDeployment();
    }

    /** Tests the deployment of multiple nodes. */
    @Override
    @Test
    void testMultiNodeDeployment() throws Exception {
        super.testMultiNodeDeployment();
    }

    /**
     * Prepares a builder for a consensus capsule with specific configurations for secret hostpath.
     *
     * @param node the node information
     * @return the prepared ConsensusCapsule builder
     */
    @Override
    protected ConsensusCapsule prepareConsensusCapsuleBuilder(final TestRecords.NodeInfo node) {
        return edit(super.prepareConsensusCapsuleBuilder(node))
                .with(spec -> spec.editSpec()
                        .withExternalAddress("not-found")
                        .withNewConfigFolder("/tmp/" + namespace + "/dst-config", "Directory")
                        .editSecrets()
                        .withSecretResources(null)
                        .withNewHostPath("/tmp/" + namespace + "/" + node.name() + "-dst", "Directory"))
                .build();
    }

    /**
     * Checks the address book using Hedera SDK client context factory.
     *
     * @param hederaSdkClientContextFactory the Hedera SDK client context factory
     * @throws AssertionError if assertion fails
     * @throws Exception if an error occurs
     */
    @Override
    protected void checkAddressBook(final HederaSdkClientContextFactory hederaSdkClientContextFactory)
            throws AssertionError, Exception {
        super.checkAddressBook(HederaSdkClientFactory.directContextCreator(certificates, pubKeys));
    }

    /**
     * Runs a command in a pod with specified volumes and volume mounts.
     *
     * @param volumes the list of volumes
     * @param volumeMounts the list of volume mounts
     * @param command the command to run
     */
    void runInPod(final List<Volume> volumes, final List<VolumeMount> volumeMounts, final String command) {
        Pod pod = createPod(volumes, volumeMounts, command);
        await().atMost(Duration.ofSeconds(150)).untilAsserted(() -> {
            Pod tmpPod = kubernetesClient.resource(pod).get();
            assertThat(tmpPod).isNotNull();
            assertThat(tmpPod.getStatus().getPhase()).isEqualTo("Succeeded");
        });
        kubernetesClient.resource(pod).delete();
    }

    /**
     * Creates a pod with specified volumes, volume mounts, and run specified command in it.
     *
     * @param volumes the list of volumes
     * @param volumeMounts the list of volume mounts
     * @param command the command to run
     * @return the created Pod object
     */
    Pod createPod(final List<Volume> volumes, final List<VolumeMount> volumeMounts, final String command) {
        return operator.getKubernetesClient()
                .resource(edit(new Pod())
                        .with(metadata ->
                                metadata.editMetadata().withGenerateName("pod-").withNamespace(namespace))
                        .with(spec -> spec.editSpec().addAllToVolumes(volumes))
                        .with(container -> container
                                .editSpec()
                                .withRestartPolicy("Never")
                                .addNewContainer()
                                .withName("main")
                                .withImage("alpine")
                                .withCommand("sh", "-c", command)
                                .addAllToVolumeMounts(volumeMounts))
                        .build())
                .create();
    }

    /**
     * Creates a ConfigMap in the Kubernetes cluster.
     *
     * @param configMap the ConfigMap to create
     */
    void createConfigMap(final ConfigMap configMap) {
        configMaps.add(kubernetesClient
                .resource(configMap
                        .edit()
                        .editMetadata()
                        .withNamespace(namespace)
                        .endMetadata()
                        .build())
                .create());
    }

    /**
     * Adds volumes and volume mounts for a ConfigMap.
     *
     * @param srcName the source volume name
     * @param dstName the destination volume name
     * @param configMap the ConfigMap to add volumes and volume mounts for
     */
    void addVolumes(final String srcName, final String dstName, final ConfigMap configMap) {
        String cmName = configMap.getMetadata().getName();
        volumes.add(new VolumeBuilder()
                .withName(srcName)
                .editConfigMap()
                .withName(cmName)
                .endConfigMap()
                .build());
        volumes.add(new VolumeBuilder()
                .withName(dstName)
                .editHostPath()
                .withType("DirectoryOrCreate")
                .withPath("/tmp/" + namespace + "/" + dstName)
                .endHostPath()
                .build());
        volumeMounts.add(new VolumeMountBuilder()
                .withName(srcName)
                .withMountPath("/mnt/src/" + cmName)
                .build());
        volumeMounts.add(new VolumeMountBuilder()
                .withName(dstName)
                .withMountPath("/mnt/dst/" + cmName)
                .build());
    }
}
