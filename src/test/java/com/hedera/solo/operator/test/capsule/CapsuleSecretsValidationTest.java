// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.capsule;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.operator.test.fixtures.coredns.ConfigureCoredns.configureCoredns;
import static java.util.Collections.emptyMap;
import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;
import static org.junit.jupiter.api.Assertions.assertThrows;

import com.hedera.solo.operator.beans.DefaultConsensusNodeBean;
import com.hedera.solo.operator.library.builder.BuilderFunction;
import com.hedera.solo.operator.model.commonspec.KeyReference;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsuleBuilder;
import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.secrets.SecretResources;
import com.hedera.solo.operator.reconciler.common.ConditionStatus;
import com.hedera.solo.operator.reconciler.common.Constants;
import com.hedera.solo.operator.test.fixtures.Resources;
import com.hedera.solo.operator.test.fixtures.annotations.SoloTest;
import com.hedera.solo.operator.test.fixtures.snapshot.SnapshotFailureContext;
import io.fabric8.kubernetes.api.model.Condition;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.fabric8.kubernetes.client.KubernetesClientException;
import io.javaoperatorsdk.operator.Operator;
import io.quarkus.test.junit.QuarkusTest;
import jakarta.inject.Inject;
import java.time.Duration;
import java.util.Map;
import java.util.function.Consumer;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

/** This class contains tests for the validation of secrets in a ConsensusCapsule. */
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

    /** Set up the test environment before each test. */
    @BeforeEach
    public void setup() {
        configureCoredns(kubernetesClient);
        Orbit orbit = kubernetesClient
                .resource(edit(new Orbit())
                        .with(metadata -> metadata.editMetadata()
                                .withGenerateName(getClass().getSimpleName().toLowerCase() + "-")
                                .withLabels(Map.of(Constants.LABEL_K8S_MANAGED_BY, Constants.SOLO_OPERATOR)))
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

    /**
     * Clean up the test environment after each test.
     *
     * @param snapshot - The context for snapshot failure handling.
     */
    @AfterEach
    void afterEach(final SnapshotFailureContext snapshot) {
        kubernetesClient
                .resources(ConsensusCapsule.class)
                .inNamespace(namespace)
                .delete();

        Resources.removeByName(kubernetesClient, Orbit.class, namespace);

        await().atMost(Duration.ofSeconds(120))
                .until(() -> kubernetesClient.namespaces().withName(namespace).get() == null);
    }

    /** Test the validation of secrets when they are valid. */
    @Test
    void testGenerateSecretsValid() {
        ConsensusCapsule capsule = createConsensusCapsule();
        test(capsule, condition -> assertThat(condition.getStatus()).isEqualTo(ConditionStatus.True));
    }

    /** Test the validation of secrets when they are invalid. */
    @Test
    void testSecretResourcesAndHostPathInvalid() {
        KubernetesClientException exception = assertThrows(
                KubernetesClientException.class,
                () -> createConsensusCapsule(
                        secret -> secret.editSpec().editSecrets().withNewHostPath("path", "")));
        assertThat(exception.getMessage())
                .contains("must validate one and only one schema (oneOf). Found 2 valid alternatives");
    }

    /**
     * Create a ConsensusCapsule with default secret specifications.
     *
     * @return - The created ConsensusCapsule
     */
    ConsensusCapsule createConsensusCapsule() {
        return createConsensusCapsule(builder -> builder);
    }

    /**
     * Create a ConsensusCapsule with custom secret specifications provided by the BuilderFunction.
     *
     * @param builder - A function to build custom secret specifications.
     * @return - The created ConsensusCapsule
     */
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
                                .withNamespace(namespace)
                                .withLabels(Map.of(
                                        Constants.LABEL_K8S_MANAGED_BY,
                                        Constants.SOLO_OPERATOR,
                                        Constants.LABEL_K8S_COMPONENT,
                                        Constants.COMPONENT_CONSENSUS_NODE)))
                        .with(spec -> spec.editSpec()
                                .withAccountId("0.0.3")
                                .withWeight(1L)
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

    /**
     * Test the conditions of a ConsensusCapsule and assert the condition based on the consumer function.
     *
     * @param consensusCapsule - The ConsensusCapsule to be tested.
     * @param consumer - A consumer function to process and verify the condition.
     */
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
