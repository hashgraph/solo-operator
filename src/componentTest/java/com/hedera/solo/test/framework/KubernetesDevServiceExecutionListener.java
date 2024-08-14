/*
 * Copyright (C) 2024 Hedera Hashgraph, LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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
import org.eclipse.microprofile.config.ConfigProvider;
import org.junit.platform.launcher.TestExecutionListener;
import org.junit.platform.launcher.TestPlan;

public class KubernetesDevServiceExecutionListener implements TestExecutionListener {
    private static final KubernetesContainer<?> container = container();

    public static LocallyRunOperatorExtension.Builder operatorExtensionBuilder() {
        return LocallyRunOperatorExtension.builder().withKubernetesClient(kubernetesClient());
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
