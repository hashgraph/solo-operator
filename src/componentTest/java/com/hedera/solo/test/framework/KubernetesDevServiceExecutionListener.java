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

public class KubernetesDevServiceExecutionListener implements TestExecutionListener {
    private static final KubernetesContainer<?> container = container();

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

    private static <T> T getConfig(String name, Class<T> type, T defaultValue) {
        return ConfigProvider.getConfig().getOptionalValue(name, type).orElse(defaultValue);
    }

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
