// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.capsule;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.operator.test.fixtures.Environment.isTestUsingGradleOrIntelliJ;
import static com.hedera.solo.operator.test.fixtures.coredns.ConfigureCoredns.configureCoredns;
import static java.util.Collections.emptyMap;
import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

import com.hedera.solo.operator.beans.DefaultConsensusNodeBean;
import com.hedera.solo.operator.library.builder.BuilderFunction;
import com.hedera.solo.operator.model.commonspec.KeyReference;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsuleBuilder;
import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.secrets.SecretResources;
import com.hedera.solo.operator.reconciler.common.ConditionStatus;
import com.hedera.solo.operator.test.fixtures.annotations.SoloTest;
import com.hedera.solo.operator.test.fixtures.snapshot.SnapshotFailureContext;
import io.fabric8.kubernetes.api.model.Condition;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.javaoperatorsdk.operator.Operator;
import io.quarkus.test.junit.QuarkusTest;
import jakarta.inject.Inject;
import java.time.Duration;
import java.util.function.Consumer;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

@QuarkusTest
@SoloTest
public class CapsuleSecretsValidationTest {
    @Inject
    KubernetesClient kubernetesClient;

    @Inject
    Operator operator;

    @Inject
    DefaultConsensusNodeBean defaultConsensusNode;

    String namespace;

    @BeforeEach
    public void setup() {
        configureCoredns(kubernetesClient);
        if (isTestUsingGradleOrIntelliJ()) {
            operator.start();
        }
        Orbit orbit = kubernetesClient
                .resource(edit(new Orbit())
                        .with(metadata -> metadata.editMetadata()
                                .withGenerateName(getClass().getSimpleName().toLowerCase() + "-"))
                        .with(spec -> spec.editSpec()
                                .editConsensus()
                                .withApplicationConfiguration(emptyMap())
                                .withNetworkSettings(emptyMap())
                                .withPermissionConfiguration(emptyMap()))
                        .build())
                .create();
        namespace = orbit.getMetadata().getName();
        await().atMost(Duration.ofSeconds(30)).untilAsserted(() -> {
            assertThat(kubernetesClient.namespaces().withName(namespace).isReady())
                    .isTrue();
        });
    }

    @AfterEach
    void afterEach(final SnapshotFailureContext snapshot) {
        snapshot.client(kubernetesClient).nonNamespacedResources("operator.solo.hedera.com/v1alpha1");
        kubernetesClient.resources(Orbit.class).withName(namespace).delete();
    }

    @Test
    void testGenerateSecretsValid() {
        ConsensusCapsule capsule = createConsensusCapsule();
        test(capsule, condition -> assertThat(condition.getStatus()).isEqualTo(ConditionStatus.True));
    }

    @Test
    void testSecretResourcesAndHostPathInvalid() {
        ConsensusCapsule capsule =
                createConsensusCapsule(secret -> secret.editSpec().editSecrets().withNewHostPath("path", ""));
        test(capsule, condition -> {
            assertThat(condition.getReason()).isEqualTo("SpecSecrets");
            assertThat(condition.getStatus()).isEqualTo(ConditionStatus.False);
        });
    }

    ConsensusCapsule createConsensusCapsule() {
        return createConsensusCapsule(builder -> builder);
    }

    ConsensusCapsule createConsensusCapsule(final BuilderFunction<ConsensusCapsuleBuilder> builder) {
        KeyReference keyReference = edit(new KeyReference())
                .with(secret -> secret.withSecretName("secret").withKeyName("key"))
                .build();
        SecretResources secretResources = edit(new SecretResources())
                .with(secret -> secret.withGrpcTlsPrivateKey(keyReference)
                        .withGrpcTlsPublicCertificate(keyReference)
                        .withSigningPrivateKey(keyReference)
                        .withSigningPublicCertificate(keyReference))
                .build();
        return kubernetesClient
                .resource(edit(new ConsensusCapsule())
                        .with(metadata -> metadata.editMetadata()
                                .withGenerateName("test-")
                                .withNamespace(namespace))
                        .with(spec -> spec.editSpec()
                                .withAccountId("0.0.3")
                                .withLog4j2Xml("")
                                .withNodeId(1L)
                                .withOrbit(namespace))
                        .with(consensus -> consensus
                                .editSpec()
                                .editPodProperties()
                                .editContainers()
                                .withConsensusNode(defaultConsensusNode.defaultConsensusNode()))
                        .with(secret -> secret.editSpec().editSecrets().withSecretResources(secretResources))
                        .with(builder)
                        .build())
                .create();
    }

    void test(final ConsensusCapsule consensusCapsule, final Consumer<Condition> consumer) {
        await().atMost(Duration.ofSeconds(90)).untilAsserted(() -> {
            ConsensusCapsule capsule =
                    kubernetesClient.resource(consensusCapsule).get();
            assertThat(capsule).isNotNull();
            assertThat(capsule.getStatus()).isNotNull();
            assertThat(capsule.getStatus().getConditions()).isNotNull();
            assertThat(capsule.getStatus().getConditions()).size().isEqualTo(1);
            Condition condition = capsule.getStatus().getConditions().getFirst();
            assertThat(condition.getType()).isEqualTo("ManifestValidation");
            consumer.accept(condition);
        });
    }
}
