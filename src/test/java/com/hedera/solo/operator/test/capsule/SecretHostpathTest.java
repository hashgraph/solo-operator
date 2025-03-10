// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.capsule;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;
import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

import com.hedera.solo.operator.library.builder.BuilderFunction;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.test.fixtures.hederaclient.HederaSdkClientContextFactory;
import com.hedera.solo.operator.test.fixtures.hederaclient.HederaSdkClientFactory;
import com.hedera.solo.operator.test.utils.TestRecords;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.Pod;
import io.fabric8.kubernetes.api.model.PodBuilder;
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

@QuarkusTest
public class SecretHostpathTest extends ConsensusCapsuleTestAbstract {
    boolean setup;
    List<ConfigMap> configMaps = new ArrayList<>();
    Map<String, byte[]> certificates = new HashMap<>();
    Map<String, String> pubKeys = new HashMap<>();

    @Override
    @BeforeEach
    void setup() {
        setup = true;
        try {
            super.setup();
            for (int i = 1; i <= 3; ++i) {
                String nodeName = "node" + i;
                ConfigMap configMap = loadYaml(ConfigMap.class, getClass(), "secrethostpath/" + nodeName + "-src.yaml");
                certificates.put(nodeName, configMap.getData().get("hedera.crt").getBytes());
                pubKeys.put(nodeName, configMap.getData().get("s-public-" + nodeName + ".pem"));
                configMaps.add(kubernetesClient
                        .resource(configMap
                                .edit()
                                .editMetadata()
                                .withNamespace(namespace)
                                .endMetadata()
                                .build())
                        .create());
            }
            runInPod(
                    "find -L /mnt/src -maxdepth 2 -mindepth 2 -type f -print | sed 's,\\(/mnt/src/\\(.*\\)\\),cp \\1 /mnt/dst/\\2,' | sh");
        } finally {
            setup = false;
        }
    }

    @Override
    void cleanup() {
        if (!setup) {
            runInPod("find /mnt/dst -type f -exec rm {} \\;");
            configMaps.forEach(configMap -> kubernetesClient.resource(configMap).delete());
        }
        super.cleanup();
    }

    @Override
    @Disabled("not required")
    void testSingleNodeDeployment() throws Exception {
        super.testSingleNodeDeployment();
    }

    @Override
    @Test
    void testMultiNodeDeployment() throws Exception {
        super.testMultiNodeDeployment();
    }

    @Override
    protected ConsensusCapsule prepareConsensusCapsuleBuilder(final TestRecords.NodeInfo node) {
        return edit(super.prepareConsensusCapsuleBuilder(node))
                .with(spec -> spec.editSpec()
                        .editSecrets()
                        .withSecretResources(null)
                        .withNewHostPath("/tmp/" + namespace + "-" + node.name() + "-dst", "Directory"))
                .build();
    }

    @Override
    protected void checkAddressBook(final HederaSdkClientContextFactory hederaSdkClientContextFactory)
            throws AssertionError, Exception {
        super.checkAddressBook(HederaSdkClientFactory.directContextCreator(certificates, pubKeys));
    }

    void runInPod(final String command) {
        Pod pod = createPod(command);
        await().atMost(Duration.ofSeconds(150)).untilAsserted(() -> {
            Pod tmpPod = kubernetesClient.resource(pod).get();
            assertThat(tmpPod).isNotNull();
            assertThat(tmpPod.getStatus().getPhase()).isEqualTo("Succeeded");
        });
        kubernetesClient.resource(pod).delete();
    }

    Pod createPod(final String command) {
        return operator.getKubernetesClient()
                .resource(edit(new Pod())
                        .with(metadata ->
                                metadata.editMetadata().withGenerateName("pod-").withNamespace(namespace))
                        .with(addConfigMap("node1-src"))
                        .with(addConfigMap("node2-src"))
                        .with(addConfigMap("node3-src"))
                        .with(addHostPath("node1-dst"))
                        .with(addHostPath("node2-dst"))
                        .with(addHostPath("node3-dst"))
                        .with(container -> container
                                .editSpec()
                                .withRestartPolicy("Never")
                                .addNewContainer()
                                .withName("main")
                                .withImage("alpine")
                                .withCommand("sh", "-c", command)
                                .addAllToVolumeMounts(volumeMounts()))
                        .build())
                .create();
    }

    BuilderFunction<PodBuilder> addConfigMap(final String name) {
        return builder ->
                builder.editSpec().addNewVolume().withName(name).editConfigMap().withName(name);
    }

    BuilderFunction<PodBuilder> addHostPath(final String name) {
        return builder -> builder.editSpec()
                .addNewVolume()
                .withName(name)
                .editHostPath()
                .withType("DirectoryOrCreate")
                .withPath("/tmp/" + namespace + "-" + name);
    }

    List<VolumeMount> volumeMounts() {
        List<VolumeMount> volumeMounts = new ArrayList<>();
        for (int i = 1; i <= 3; ++i) {
            volumeMounts.add(new VolumeMountBuilder()
                    .withName("node" + i + "-src")
                    .withMountPath("/mnt/src/" + i)
                    .build());
            volumeMounts.add(new VolumeMountBuilder()
                    .withName("node" + i + "-dst")
                    .withMountPath("/mnt/dst/" + i)
                    .build());
        }
        return volumeMounts;
    }
}
