// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.envoy;

import static com.hedera.solo.operator.library.render.QuteUtils.template;
import static java.util.function.Predicate.not;
import static org.assertj.core.api.Assertions.assertThat;
import static org.awaitility.Awaitility.await;

import com.hedera.solo.operator.model.v1alpha1.EnvoyProxy;
import com.hedera.solo.operator.test.fixtures.fuzz.BeanFactory;
import com.hedera.solo.operator.test.fixtures.kubernetes.TempNamespace;
import com.hedera.solo.operator.test.fixtures.microcks.MicrocksService;
import com.hedera.solo.operator.test.framework.MicrocksManager;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.fabric8.kubernetes.client.utils.KubernetesSerialization;
import io.quarkus.test.junit.QuarkusTest;
import jakarta.inject.Inject;
import java.time.Duration;
import java.util.stream.IntStream;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;

/**
 * Abstract base class for tests involving EnvoyProxy resources in a Kubernetes environment. It sets up a temporary
 * namespace and provides methods to create and manage EnvoyProxy resources.
 */
@QuarkusTest
public abstract class EnvoyTestAbstract {
    @Inject
    protected KubernetesClient kubernetesClient;

    protected final BeanFactory beanFactory = new BeanFactory()
            .charset(IntStream.range('0', 'z' + 1)
                    .filter(c -> ('0' <= c && c <= '9') || ('a' <= c && c <= 'z'))
                    .mapToObj(Character::toString)
                    .toList());
    protected final KubernetesSerialization kubernetesSerialization = new KubernetesSerialization();
    private TempNamespace tempNamespace;
    MicrocksService microcksService;
    EnvoyProxy envoyProxy;

    /**
     * Injects the MicrocksManager to access the MicrocksService.
     *
     * @param microcksManager the MicrocksManager instance
     */
    @Inject
    void setMicrocksManager(final MicrocksManager microcksManager) {
        this.microcksService = microcksManager.getMicrocksService();
    }

    /** Sets up the test environment by creating a temporary namespace and an EnvoyProxy resource. */
    @BeforeEach
    void setUp() {
        tempNamespace = new TempNamespace(kubernetesClient, getClass());
        envoyProxy = createEnvoyProxy();
    }

    /** Cleans up the resources after each test by deleting the EnvoyProxy resource and the temporary namespace. */
    @AfterEach
    void tearDown() {
        kubernetesClient.resources(EnvoyProxy.class).inNamespace(namespace()).delete();
        await().atMost(Duration.ofSeconds(60)).until(() -> kubernetesClient
                .resources(EnvoyProxy.class)
                .inNamespace(namespace())
                .list()
                .getItems()
                .isEmpty());
        tempNamespace.close();
    }

    /**
     * Returns the name of the temporary namespace used for testing.
     *
     * @return the name of the temporary namespace
     */
    String namespace() {
        return tempNamespace.name();
    }

    /**
     * Creates an EnvoyProxy resource in the Kubernetes cluster.
     *
     * @return the created EnvoyProxy resource
     */
    EnvoyProxy createEnvoyProxy() {
        Integer[] ports = randomDistinctPorts(2);
        EnvoyProxy created = kubernetesClient
                .resource(kubernetesSerialization.unmarshal(
                        template("test/envoy/envoy-proxy.yaml")
                                .instance()
                                .data("proxyPort", ports[0])
                                .data("adminPort", ports[1])
                                .data("name", "envoy-" + randomNonEmptyString())
                                .data("microcksNamespace", microcksService.namespace())
                                .render(),
                        EnvoyProxy.class))
                .inNamespace(namespace())
                .create();
        await().atMost(Duration.ofSeconds(60)).untilAsserted(() -> {
            EnvoyProxy response = kubernetesClient.resource(created).get();
            assertThat(response).isNotNull();
            assertThat(response.getStatus()).isNotNull();
            assertThat(response.getStatus().getAvailableReplicas()).isGreaterThanOrEqualTo(1);
        });
        return created;
    }

    /**
     * Generates an array of random distinct ports.
     *
     * @param count the number of distinct ports to generate
     * @return an array of random distinct ports
     */
    Integer[] randomDistinctPorts(final int count) {
        return beanFactory
                .nonNulls(Integer.class)
                .filter(i -> 1 <= i && i <= 0xffff)
                .distinct()
                .limit(count)
                .toArray(Integer[]::new);
    }

    /**
     * Generates a random non-empty string from the bean factory's non-null strings.
     *
     * @return a random non-empty string
     */
    String randomNonEmptyString() {
        return beanFactory
                .nonNulls(String.class)
                .filter(not(String::isEmpty))
                .findFirst()
                .orElseThrow();
    }
}
