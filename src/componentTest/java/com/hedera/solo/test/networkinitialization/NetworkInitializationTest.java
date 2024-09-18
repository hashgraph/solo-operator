package com.hedera.solo.test.networkinitialization;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.operator.model.v1alpha1.NetworkInitializationSpec.Mode.FirstInitialization;
import static com.hedera.solo.operator.reconciler.common.Metadata.OPERATOR_PREFIX;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitialization;
import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.reconciler.networkinitialization.NetworkInitializationReconciler;
import com.hedera.solo.test.framework.AbstractOperatorTest;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.javaoperatorsdk.operator.junit.LocallyRunOperatorExtension;
import java.util.List;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

public class NetworkInitializationTest extends AbstractOperatorTest {
    @Override
    protected LocallyRunOperatorExtension.Builder configure(final KubernetesClient client) {
        return builder(client).withReconciler(new NetworkInitializationReconciler());
    }

    @BeforeEach
    void setup() {
        createOrbit();
    }

    @AfterEach
    void teardown() {
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
