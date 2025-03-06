// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.test.consensuscapsule.service;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.operator.model.v1alpha1.ConsensusCapsuleSpec.ServiceType.LoadBalancer;
import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

import com.fasterxml.jackson.core.type.TypeReference;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsuleSpec;
import com.hedera.solo.operator.test.fixtures.annotations.SoloTest;
import com.hedera.solo.operator.test.fixtures.fuzz.BeanFactory;
import com.hedera.solo.operator.test.fixtures.snapshot.SnapshotFailureContext;
import com.hedera.solo.test.framework.AbstractOperatorTest;
import io.fabric8.kubernetes.api.model.Service;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.javaoperatorsdk.operator.junit.LocallyRunOperatorExtension;
import java.time.Duration;
import java.util.Collections;
import java.util.Map;
import java.util.stream.Collectors;
import java.util.stream.IntStream;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.Test;

@SoloTest
public class ConsensusCapsuleServiceTest extends AbstractOperatorTest {
    BeanFactory factory = new BeanFactory()
            .nullProbability(0)
            .charset(IntStream.range('0', 'z' + 1)
                    .filter(c -> ('0' <= c && c <= '9') || ('A' <= c && c <= 'Z') || ('a' <= c && c <= 'z'))
                    .mapToObj(Character::toString)
                    .collect(Collectors.toList()));

    @Override
    protected LocallyRunOperatorExtension.Builder configure(final KubernetesClient client) {
        return builder(client).withReconciler(new ConsensusCapsuleServiceReconciler());
    }

    @AfterEach
    void afterEach(final SnapshotFailureContext snapshot) {
        snapshot.client(operator().getKubernetesClient())
                .namespaces(operator().getNamespace())
                .namespacedResources();
    }

    @Test
    void testDefault() {
        ConsensusCapsule capsule = consensusCapsule(null, null);
        verify(capsule, LoadBalancer, Collections.emptyMap());
    }

    @Test
    void testRandomized() {
        ConsensusCapsuleSpec.ServiceType serviceType = factory.random(ConsensusCapsuleSpec.ServiceType.class);
        Map<String, String> annotations = factory.random(new TypeReference<Map<String, String>>() {});
        annotations.remove("");
        ConsensusCapsule capsule = consensusCapsule(serviceType, annotations);
        verify(capsule, serviceType, annotations);
    }

    ConsensusCapsule consensusCapsule(ConsensusCapsuleSpec.ServiceType serviceType, Map<String, String> annotations) {
        return operator()
                .create(edit(new ConsensusCapsule())
                        .with(metadata -> metadata.editMetadata().withGenerateName("cc-"))
                        .with(spec -> spec.editSpec()
                                .withAccountId("0")
                                .withWeight(1L)
                                .withLog4j2Xml("log")
                                .withNodeId(0L)
                                .withOrbit("orbit")
                                .editPodProperties()
                                .editContainers()
                                .editConsensusNode()
                                .editSoftwareVersion()
                                .withRepository("repository")
                                .withImageName("image")
                                .withImageTag("tag"))
                        .with(builder -> annotations == null
                                ? builder
                                : builder.editMetadata().withAnnotations(annotations))
                        .with(builder -> serviceType == null
                                ? builder
                                : builder.editSpec().withServiceType(serviceType))
                        .build());
    }

    void verify(
            ConsensusCapsule capsule, ConsensusCapsuleSpec.ServiceType serviceType, Map<String, String> annotations) {
        await().atMost(Duration.ofSeconds(30)).untilAsserted(() -> {
            ConsensusCapsule cc =
                    operator().get(ConsensusCapsule.class, capsule.getMetadata().getName());
            assertThat(cc).isNotNull();
            assertThat(cc.getSpec().getServiceType()).isEqualTo(serviceType);
            assertThat(cc.getMetadata().getAnnotations()).isEqualTo(annotations);
            Service service = operator().get(Service.class, cc.getMetadata().getName() + "-svc");
            assertThat(service).isNotNull();
        });
    }
}
