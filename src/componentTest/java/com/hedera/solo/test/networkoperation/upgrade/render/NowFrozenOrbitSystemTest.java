// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.test.networkoperation.upgrade.render;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.test.framework.KubernetesDevServiceExecutionListener.initialize;
import static com.hedera.solo.test.networkoperation.upgrade.render.SystemTestUtils.create;
import static com.hedera.solo.test.networkoperation.upgrade.render.SystemTestUtils.createNamespace;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.reconciler.networkoperation.upgrade.steps.ApplyOrbitConfigurationChangeStep;
import com.hedera.solo.operator.test.fixtures.fuzz.BeanFactory;
import com.hedera.solo.test.networkoperation.NoOpOrbitReconciler;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.junit.AbstractOperatorExtension;
import io.javaoperatorsdk.operator.junit.LocallyRunOperatorExtension;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Disabled;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.RegisterExtension;

@Disabled("need to revisit this fuzz test later")
public class NowFrozenOrbitSystemTest extends NowFrozenOrbitUnitTest {
    Orbit orbit;
    String namespace;
    KubernetesClient kubernetesClient;

    @RegisterExtension
    private AbstractOperatorExtension operator = initialize(this::configure);

    protected LocallyRunOperatorExtension.Builder configure(final KubernetesClient client) {
        return LocallyRunOperatorExtension.builder()
                .withKubernetesClient(client)
                .withReconciler(NoOpNetworkOperationReconciler.class)
                .withReconciler(NoOpOrbitReconciler.class);
    }

    @Override
    BeanFactory beanFactory() {
        return new SystemTestBeanFactory();
    }

    @BeforeEach
    public void setup() {
        kubernetesClient = operator.getKubernetesClient();
        namespace = createNamespace(kubernetesClient, getClass());
    }

    @AfterEach
    public void cleanup() {
        kubernetesClient.resource(orbit).delete();
        kubernetesClient.namespaces().withName(namespace).delete();
    }

    @Test
    @Override
    public void test() {
        super.test();
    }

    @Override
    Orbit update(final NetworkOperation networkOperation, final Orbit initial) {
        @SuppressWarnings("unchecked")
        Context<NetworkOperation> context = mock(Context.class);
        when(context.getClient()).thenReturn(kubernetesClient);
        ApplyOrbitConfigurationChangeStep applyOrbitConfigurationChangeStep = new ApplyOrbitConfigurationChangeStep();
        applyOrbitConfigurationChangeStep.run(networkOperation, networkOperation.getStatus(), context);
        return kubernetesClient.resource(initial).get();
    }

    @Override
    NetworkOperation networkOperation(final Orbit orbit) {
        return create(
                kubernetesClient,
                edit(super.networkOperation(orbit))
                        .with(metadata -> metadata.editMetadata()
                                .withName(null)
                                .withNamespace(namespace)
                                .withGenerateName("no-"))
                        .build());
    }

    @Override
    Orbit orbit() {
        return orbit = create(
                kubernetesClient,
                edit(super.orbit())
                        .with(metadata -> metadata.editMetadata()
                                .withName(null)
                                .withNamespace(null)
                                .withGenerateName("orbit-"))
                        .build());
    }
}
