package com.hedera.solo.test.networkinitialization;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.operator.model.v1alpha1.NetworkInitializationSpec.Mode.FirstInitialization;
import static com.hedera.solo.operator.reconciler.common.Metadata.OPERATOR_PREFIX;
import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsuleFluent;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitialization;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitializationStatus;
import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.reconciler.networkinitialization.NetworkInitializationReconciler;
import com.hedera.solo.test.framework.AbstractOperatorTest;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.javaoperatorsdk.operator.api.config.ControllerConfigurationOverrider;
import io.javaoperatorsdk.operator.junit.LocallyRunOperatorExtension;
import java.lang.invoke.MethodHandles;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.concurrent.TimeUnit;
import java.util.stream.Collectors;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.RepeatedTest;
import org.junit.jupiter.api.Test;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

public class NetworkInitializationTest extends AbstractOperatorTest {
    Logger log = LoggerFactory.getLogger(MethodHandles.lookup().lookupClass());

    @Override
    protected LocallyRunOperatorExtension.Builder configure(final KubernetesClient client) {
        return builder(client)
                .withReconciler(new NetworkInitializationTestReconciler(), ControllerConfigurationOverrider::watchingAllNamespaces)
                .withReconciler(new ConsensusCapsuleTestReconciler())
                .withReconciler(new NoOpOrbitReconciler());
    }

    @BeforeEach
    void setup() {
        createOrbit();
    }

    @AfterEach
    void teardown() {

        for (int i = 0; i < 10; ++i) {
            int cnt = i;
            NetworkInitialization ni = operator().get(NetworkInitialization.class, "init-" + cnt);
            log.info("test done {} {}", ni.getMetadata().getName(), ni.getStatus().getPhase());
        }


        log.info("deleting orbit: {}", operator().getNamespace());
        operator().getKubernetesClient().resources(Orbit.class).withName(operator().getNamespace()).delete();
    }

    @RepeatedTest(100)
    void test() {
        List<String> capsuleNames = List.of("alpha", "beta", "gamma");

        capsuleNames.forEach(this::createCapsule);

        for (int i = 0; i < 100; ++i) {
            int cnt = i;
            operator().create(edit(new NetworkInitialization())
                    .with(builder -> builder
                            .editMetadata()
                            .withName("init-" + cnt))
                    .with(builder -> builder
                            .editSpec()
                            .withMode(FirstInitialization)
                            /*.withConsensusCapsules(capsuleNames)*/)
                    .build());
//            try {
//                Thread.sleep(1500);
//            } catch (InterruptedException e) {
//                throw new RuntimeException(e);
//            }
        }

        await().atMost(30, TimeUnit.SECONDS).untilAsserted(() -> {
            ConfigMap cm = operator().get(ConfigMap.class, "cluster-config");
            assertThat(cm).isNotNull();
            String configTxt = cm.getData().get("config.txt");
            assertThat(configTxt).isNotNull();
            List<String> lines = configTxt.lines().filter(line -> line.startsWith("address")).toList();
            assertThat(lines).size().isEqualTo(3);
            Map<String, Integer> nodes = lines.stream().map(line -> line.split("\\s*,\\s*")).collect(Collectors.toMap(a -> a[2], a -> Integer.valueOf(a[1])));
            assertThat(nodes.keySet()).isEqualTo(new HashSet<>(capsuleNames));
            assertThat(new HashSet<>(nodes.values())).isEqualTo(Set.of(0, 1, 2));
        });

        await().atMost(60, TimeUnit.SECONDS).untilAsserted(() -> {
            for (int i = 0; i < 100; ++i) {
                int cnt = i;
                NetworkInitialization ni = operator().get(NetworkInitialization.class, "init-" + cnt);
                assertThat(ni).isNotNull();
                assertThat(ni.getStatus()).isNotNull();
                assertThat(ni.getStatus().getPhase()).isEqualTo(NetworkInitializationStatus.Phase.Succeeded);
            }
        });
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
                .with(ConsensusCapsuleFluent::editSpec)
                .build());
    }
}
