// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.k8s;

import com.fasterxml.jackson.core.type.TypeReference;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.ObjectReader;
import com.fasterxml.jackson.dataformat.yaml.YAMLFactory;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.quarkus.logging.Log;
import java.util.Map;

/** The KubeletConfig class provides utility methods to interact with the Kubernetes kubelet configuration. */
public class KubeletConfig {
    private static final ObjectReader reader =
            new ObjectMapper(new YAMLFactory()).readerFor(new TypeReference<Map<String, Object>>() {});

    /** Private constructor to prevent instantiation of the class. */
    private KubeletConfig() {}

    /**
     * Retrieves the cluster domain from the kubelet configuration in the Kubernetes system namespace.
     *
     * @param client The KubernetesClient instance used to interact with the Kubernetes API server.
     * @return A string representing the cluster domain, or null if it cannot be retrieved
     */
    public static String clusterDomain(final KubernetesClient client) {
        try {
            ConfigMap cm = client.configMaps()
                    .inNamespace("kube-system")
                    .withName("kubelet-config")
                    .get();
            String kubelet = cm.getData().get("kubelet");
            Map<String, Object> map = reader.readValue(kubelet);
            Object clusterDomain = map.get("clusterDomain");
            return clusterDomain instanceof String ? (String) clusterDomain : null;
        } catch (Exception exception) {
            Log.debug("Unable to read clusterDomain from KubeletConfiguration", exception);
        }
        return null;
    }
}
