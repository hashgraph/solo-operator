// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.fixtures.kubernetes;

import io.fabric8.kubernetes.client.Config;
import io.fabric8.kubernetes.client.KubernetesClient;

/**
 * The {@code KubeConfig} class provides a utility method to generate a Kubernetes configuration string based on the
 * provided Kubernetes client.
 */
public class KubeConfig {
    /**
     * Generates a Kubernetes configuration string based on the provided Kubernetes client.
     *
     * @param kubernetesClient the Kubernetes client to extract configuration from
     * @return a formatted string representing the Kubernetes configuration
     */
    public static String kubeConfig(final KubernetesClient kubernetesClient) {
        Config config = kubernetesClient.getConfiguration();
        return """
                apiVersion: v1
                clusters:
                  - name: default
                    cluster:
                      certificate-authority%s
                      server: %s
                contexts:
                  - name: default
                    context:
                      cluster: default
                      user: default
                current-context: default
                kind: Config
                preferences: {}
                users:
                  - name: default
                    user:
                      client-certificate%s
                      client-key%s
                """
                .formatted(
                        config.getCaCertData() != null
                                ? "-data: " + config.getCaCertData()
                                : ": " + config.getCaCertFile(),
                        config.getMasterUrl(),
                        config.getClientCertData() != null
                                ? "-data: " + config.getClientCertData()
                                : ": " + config.getClientCertFile(),
                        config.getClientKeyData() != null
                                ? "-data: " + config.getClientKeyData()
                                : ": " + config.getClientKeyFile());
    }
}
