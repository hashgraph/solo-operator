// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.test.sequencer;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.operator.model.v1alpha1.NetworkOperationSpec.Mode.NetworkGenesis;
import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperationStatus;
import com.hedera.solo.operator.test.fixtures.annotations.SoloTest;
import com.hedera.solo.operator.test.fixtures.snapshot.SnapshotFailureContext;
import com.hedera.solo.test.framework.AbstractOperatorTest;
import com.hedera.solo.test.networkoperation.ConsensusCapsuleTestReconciler;
import com.hedera.solo.test.networkoperation.NoOpOrbitReconciler;
import io.fabric8.kubernetes.api.model.Namespace;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.javaoperatorsdk.operator.api.config.ControllerConfigurationOverrider;
import io.javaoperatorsdk.operator.junit.LocallyRunOperatorExtension;
import java.util.concurrent.TimeUnit;
import org.assertj.core.api.Assertions;
import org.jboss.logging.Logger;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.Test;

@SoloTest
class SequenceReconcilerTest extends AbstractOperatorTest {
    private static final Logger log = Logger.getLogger(SequenceReconcilerTest.class);

    @Override
    protected LocallyRunOperatorExtension.Builder configure(final KubernetesClient client) {
        return builder(client)
                .withReconciler(
                        new SequenceReconciler(log, client), ControllerConfigurationOverrider::watchingAllNamespaces)
                .withReconciler(new ConsensusCapsuleTestReconciler())
                .withReconciler(new NoOpOrbitReconciler());
    }

    @AfterEach
    void teardown(final SnapshotFailureContext snapshot) {
        operator()
                .getKubernetesClient()
                .resources(Namespace.class)
                .withName(operator().getNamespace())
                .delete();
    }

    @Test
    void testReconcile() throws InterruptedException {
        final int total = 4;
        for (int i = 0; i < total; ++i) {
            final String name = "no-" + i;
            //            Thread.sleep(new Random().nextInt(3000));
            NetworkOperation no = createNetworkOperation(name);
            Assertions.assertThat(no).isNotNull();
        }

        await().atMost(30, TimeUnit.SECONDS).untilAsserted(() -> {
            for (int i = 0; i < total; ++i) {
                final String name = "no-" + i;
                NetworkOperation no = operator().get(NetworkOperation.class, name);
                assertThat(no).isNotNull();
                assertThat(no.getStatus()).isNotNull();
                assertThat(no.getStatus().getPhase()).isEqualTo(NetworkOperationStatus.Phase.Succeeded);
            }
        });
    }

    NetworkOperation createNetworkOperation(final String name) {
        final String orbitName = operator().getNamespace(); // for now namespace and orbit name are the same
        final NetworkOperation no = edit(new NetworkOperation())
                .with(metadata -> metadata.editMetadata().withName(name))
                .with(spec -> spec.editSpec().withMode(NetworkGenesis).withOrbit(orbitName))
                .build();
        return operator().create(no);
    }
}
