package com.hedera.solo.test.networkinitialization;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.operator.model.v1alpha1.NetworkInitializationSpec.Mode.FirstInitialization;
import static com.hedera.solo.operator.reconciler.common.Metadata.OPERATOR_PREFIX;
import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitialization;
import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.reconciler.networkinitialization.NetworkInitializationReconciler;
import com.hedera.solo.test.framework.AbstractOperatorTest;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.javaoperatorsdk.operator.api.config.ControllerConfigurationOverrider;
import io.javaoperatorsdk.operator.junit.LocallyRunOperatorExtension;
import java.lang.invoke.MethodHandles;
import java.util.List;
import java.util.concurrent.TimeUnit;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

public class NetworkInitializationTest extends AbstractOperatorTest {
    Logger log = LoggerFactory.getLogger(MethodHandles.lookup().lookupClass());

    @Override
    protected LocallyRunOperatorExtension.Builder configure(final KubernetesClient client) {
        return builder(client).withReconciler(new NetworkInitializationReconciler(), ControllerConfigurationOverrider::watchingAllNamespaces);
    }

    @BeforeEach
    void setup() {
        createOrbit();
    }

    @AfterEach
    void teardown() {
        log.info("deleting orbit: {}", operator().getNamespace());
        operator().getKubernetesClient().resources(Orbit.class).withName(operator().getNamespace()).delete();
    }

    @Test
    void test() {
        List<String> capsuleNames = List.of("alpha", "beta", "gamma");

        capsuleNames.forEach(this::createCapsule);

        operator().create(edit(new NetworkInitialization())
                .with(builder -> builder
                        .editMetadata()
                        .withName("init"))
                .with(builder -> builder
                        .editSpec()
                        .withMode(FirstInitialization)
                        .withConsensusCapsules(capsuleNames))
                .build());

        await().atMost(30, TimeUnit.SECONDS).untilAsserted(() -> {
            assertThat(operator().get(ConfigMap.class, "cluster-config")).isNotNull();
        });

        ConfigMap clusterConfig = operator().get(ConfigMap.class, "cluster-config");
        int i = 1;
    }

    void createOrbit() {
        operator().getKubernetesClient().resource(edit(new Orbit())
                        .with(builder -> builder
                                .editMetadata()
                                .withName(operator().getNamespace()))
                        .build())
                .create();

        operator().getKubernetesClient().namespaces().withName(operator().getNamespace()).edit(ns -> edit(ns)
                .with(builder -> builder
                        .editMetadata()
                        .addToLabels(OPERATOR_PREFIX + "orbit-name", operator().getNamespace()))
                .build());
    }

    void createCapsule(String name) {
        operator().create(edit(new ConsensusCapsule())
                .with(builder -> builder
                        .editMetadata()
                        .withName(name))
                .build());
    }
}
