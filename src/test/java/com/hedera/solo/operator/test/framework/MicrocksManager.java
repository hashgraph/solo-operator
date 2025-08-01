// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.framework;

import static org.awaitility.Awaitility.await;

import com.hedera.solo.operator.test.fixtures.kubernetes.TempNamespace;
import com.hedera.solo.operator.test.fixtures.microcks.EchoService;
import com.hedera.solo.operator.test.fixtures.microcks.MicrocksService;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.fabric8.kubernetes.client.KubernetesClientBuilder;
import io.fabric8.kubernetes.client.utils.KubernetesSerialization;
import io.quarkus.runtime.ShutdownEvent;
import io.quarkus.runtime.StartupEvent;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.enterprise.event.Observes;
import java.time.Duration;
import java.util.ArrayDeque;
import java.util.Deque;
import java.util.List;
import org.awaitility.core.ConditionTimeoutException;

/**
 * MicrocksManager is responsible for managing the Microcks service lifecycle within a Kubernetes environment. It
 * deploys the Microcks service in a temporary namespace and provides methods to access it.
 */
@ApplicationScoped
public class MicrocksManager {
    private KubernetesClient kubernetesClient;
    private MicrocksService microcksService;
    private TempNamespace tempNamespace;

    /**
     * Initializes the Microcks service by deploying it in a temporary namespace.
     *
     * @param event the startup event
     * @param client the Kubernetes client to interact with the cluster
     * @throws Exception if an error occurs during initialization
     */
    void onStart(@Observes final StartupEvent event, final KubernetesClient client) throws Exception {
        KubernetesSerialization kubernetesSerialization = new KubernetesSerialization();
        kubernetesClient = new KubernetesClientBuilder()
                .withConfig(kubernetesSerialization.asYaml(client.getConfiguration()))
                .build();
        tempNamespace = new TempNamespace(kubernetesClient, "microcks-");
        microcksService = new MicrocksService(kubernetesClient, tempNamespace.name());
        try {
            Deque<List<String>> resources = new ArrayDeque<>();
            resources.addAll(EchoService.resources());
            await().atMost(Duration.ofSeconds(90)).ignoreExceptions().until(() -> {
                List<String> next = resources.peek();
                if (next == null) {
                    return true;
                }
                microcksService.upload(
                        next.stream().map(ClassLoader::getSystemResource).toList());
                resources.remove();
                return resources.isEmpty();
            });
        } catch (ConditionTimeoutException e) {
            microcksService.close();
            microcksService = null;
            throw e;
        }
    }

    /**
     * Cleans up resources when the application is shutting down.
     *
     * @param event the shutdown event
     * @throws Exception if an error occurs during cleanup
     */
    void onShutdown(@Observes final ShutdownEvent event) throws Exception {
        if (microcksService != null) {
            microcksService.close();
            microcksService = null;
        }
        if (tempNamespace != null) {
            tempNamespace.close();
            tempNamespace = null;
        }
        if (kubernetesClient != null) {
            kubernetesClient.close();
            kubernetesClient = null;
        }
    }

    /**
     * Returns the namespace where the Microcks service is deployed.
     *
     * @return the namespace as a string
     */
    public MicrocksService getMicrocksService() {
        return microcksService;
    }
}
