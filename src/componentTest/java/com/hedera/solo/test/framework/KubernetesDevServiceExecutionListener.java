// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.test.framework;

import static io.fabric8.kubernetes.client.Config.fromKubeconfig;

import com.dajudge.kindcontainer.ApiServerContainer;
import com.dajudge.kindcontainer.K3sContainer;
import com.dajudge.kindcontainer.KindContainer;
import com.dajudge.kindcontainer.KubernetesContainer;
import com.dajudge.kindcontainer.KubernetesWithKubeletContainer;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.fabric8.kubernetes.client.KubernetesClientBuilder;
import io.javaoperatorsdk.operator.junit.LocallyRunOperatorExtension;
import java.util.Collections;
import java.util.function.Function;
import org.eclipse.microprofile.config.ConfigProvider;
import org.junit.platform.launcher.TestExecutionListener;
import org.junit.platform.launcher.TestPlan;

/**
 * This class implements a custom test execution listener for JUnit that manages a Kubernetes container for local
 * development.
 */
public class KubernetesDevServiceExecutionListener implements TestExecutionListener {
    private static final KubernetesContainer<?> container = container();

    /**
     * Initializes the operator extension with a configured Kubernetes client.
     *
     * @param configure - A function to configure the Kubernetes client and build the LocallyRunOperatorExtension.
     * @return The initialized LocallyRunOperatorExtension
     */
    public static LocallyRunOperatorExtension initialize(
            final Function<KubernetesClient, LocallyRunOperatorExtension.Builder> configure) {
        return configure.apply(kubernetesClient()).build();
    }

    @Override
    public void testPlanExecutionStarted(TestPlan testPlan) {
        if (null != container) {
            container.start();
            System.out.println();
            System.out.println("-----BEGIN CONTAINER KUBECONFIG-----");
            System.out.print(container.getKubeconfig());
            System.out.println("-----END CONTAINER KUBECONFIG-----");
            System.out.println();
        }
    }

    @Override
    public void testPlanExecutionFinished(TestPlan testPlan) {
        if (null != container) {
            container.stop();
        }
    }

    /**
     * Creates a Kubernetes container based on the configuration properties. It checks if Dev Services are enabled and
     * which flavor of Kubernetes to use.
     *
     * @return A KubernetesContainer instance or null if Dev Services are not enabled
     */
    private static KubernetesContainer<?> container() {
        boolean enabled = getConfig("quarkus.kubernetes-client.devservices.enabled", Boolean.class, true);
        boolean override = getConfig("quarkus.kubernetes-client.devservices.override-kubeconfig", Boolean.class, false);
        String flavor = getConfig("quarkus.kubernetes-client.devservices.flavor", String.class, "api-only");
        if (!enabled || !override) {
            return null;
        }
        if ("kind".equals(flavor)) {
            return new KindContainer<>();
        }
        if ("k3s".equals(flavor)) {
            KubernetesWithKubeletContainer<?> container = new K3sContainer<>();
            container.setBinds(Collections.emptyList());
            return container;
        }
        if ("api-only".equals(flavor)) {
            return new ApiServerContainer<>();
        }
        throw new IllegalArgumentException();
    }

    /**
     * Retrieves a configuration value from the Microprofile ConfigProvider.
     *
     * @param <T> - The type of the configuration value
     * @param name - The name of the configuration property.
     * @param type - The class type of the configuration value.
     * @param defaultValue - The default value to return if the configuration is not found.
     * @return The configuration value or the default value if it does not exist
     */
    private static <T> T getConfig(String name, Class<T> type, T defaultValue) {
        return ConfigProvider.getConfig().getOptionalValue(name, type).orElse(defaultValue);
    }

    /**
     * Creates and returns a Kubernetes client configured with the container's kubeconfig if available.
     *
     * @return A KubernetesClient instance
     */
    private static KubernetesClient kubernetesClient() {
        if (null == container) {
            return new KubernetesClientBuilder().build();
        } else {
            return new KubernetesClientBuilder()
                    .withConfig(fromKubeconfig(container.getKubeconfig()))
                    .build();
        }
    }
}
