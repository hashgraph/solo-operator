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

package com.hedera.solo.operator.test;

import com.dajudge.kindcontainer.K3sContainer;
import com.dajudge.kindcontainer.client.KubeConfigUtils;
import com.dajudge.kindcontainer.client.config.KubeConfig;
import com.hedera.solo.operator.reconciler.orbit.OrbitReconciler;
import io.fabric8.kubernetes.client.Config;
import io.fabric8.kubernetes.client.ConfigBuilder;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.fabric8.kubernetes.client.KubernetesClientBuilder;
import io.javaoperatorsdk.operator.junit.LocallyRunOperatorExtension;
import java.lang.invoke.MethodHandles;
import java.util.Collections;
import org.junit.jupiter.api.Disabled;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.RegisterExtension;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

@Disabled
public class KindContainerTest {
    private final Logger log = LoggerFactory.getLogger(MethodHandles.lookup().lookupClass());

    //    KindContainer<?> kind;

    Config config(String kubeConfigS) {
        KubeConfig kubeConfig = KubeConfigUtils.parseKubeConfig(kubeConfigS);
        return new ConfigBuilder()
                //                .withApiVersion(kubeConfig.getApiVersion())
                //                .withCurrentContext(new
                // NamedContextBuilder().withName(kubeConfig.getCurrentContext()).build())
                //                .withContexts(new NamedContextBuilder()
                //                        .withName(kubeConfig.getContexts().get(0).getName())
                //                        .withContext(new ContextBuilder()
                //                                .withCluster(kubeConfig.getContexts().get(0).getName())
                //                                .withUser(kubeConfig.getContexts().get(0).getContext().getUser())
                //                                .build())
                //                        .build())
                .withMasterUrl(kubeConfig.getClusters().get(0).getCluster().getServer())
                .withCaCertData(kubeConfig.getClusters().get(0).getCluster().getCertificateAuthorityData())
                .withClientCertData(kubeConfig.getUsers().get(0).getUser().getClientCertificateData())
                .withClientKeyData(kubeConfig.getUsers().get(0).getUser().getClientKeyData())
                .withClientKeyAlgo(Config.getKeyAlgorithm(
                        null, kubeConfig.getUsers().get(0).getUser().getClientKeyData()))
                //                .withClientKeyPassphrase("")
                .build();
    }

    KubernetesClient get() {
        var kind =
                //                new ApiServerContainer<>(); // namespace stuck terminating after test
                new K3sContainer<>(); // namespace stuck terminating after test
        //        kind.setDockerImageName("docker.io/rancher/k3s:v1.27.4-k3s1");
        //                new KindContainer<>();
        kind.setBinds(Collections.emptyList()); // needed for k3s
        kind.start();
        //        String yaml = kind.copyFileFromContainer("/etc/rancher/k3s/k3s.yaml", is -> new
        // String(is.readAllBytes(), StandardCharsets.UTF_8));
        KubeConfig kubeConfig = KubeConfigUtils.parseKubeConfig(kind.getKubeconfig());
        Config config = new ConfigBuilder()
                //                .withApiVersion(kubeConfig.getApiVersion())
                //                .withCurrentContext(new
                // NamedContextBuilder().withName(kubeConfig.getCurrentContext()).build())
                //                .withContexts(new NamedContextBuilder()
                //                        .withName(kubeConfig.getContexts().get(0).getName())
                //                        .withContext(new ContextBuilder()
                //                                .withCluster(kubeConfig.getContexts().get(0).getName())
                //                                .withUser(kubeConfig.getContexts().get(0).getContext().getUser())
                //                                .build())
                //                        .build())
                .withMasterUrl(kubeConfig.getClusters().get(0).getCluster().getServer())
                .withCaCertData(kubeConfig.getClusters().get(0).getCluster().getCertificateAuthorityData())
                .withClientCertData(kubeConfig.getUsers().get(0).getUser().getClientCertificateData())
                .withClientKeyData(kubeConfig.getUsers().get(0).getUser().getClientKeyData())
                .withClientKeyAlgo(Config.getKeyAlgorithm(
                        null, kubeConfig.getUsers().get(0).getUser().getClientKeyData()))
                //                .withClientKeyPassphrase("")
                .build();
        //        KubernetesSerialization kubernetesSerialization = new KubernetesSerialization();
        //        Config config = kubernetesSerialization.unmarshal(kind.getKubeconfig(), Config.class);
        // ReconcilerUtils.loadYaml(Config.class, getClass(), kind.getKubeconfig());
        return new KubernetesClientBuilder().withConfig(config).build();
    }

    @RegisterExtension
    LocallyRunOperatorExtension operator = LocallyRunOperatorExtension.builder()
            .withReconciler(new OrbitReconciler(org.jboss.logging.Logger.getLogger(OrbitReconciler.class)))
            .withKubernetesClient(get())
            .build();

    @Test
    void runKindContainer() {
        log.info("operator: {}", operator);
    }
}
