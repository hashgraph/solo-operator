// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.library.k8s;

import static com.hedera.solo.operator.reconciler.common.Constants.COMPONENT_CONFIG_MAP;
import static com.hedera.solo.operator.reconciler.common.Constants.COMPONENT_CONSENSUS_NODE;
import static com.hedera.solo.operator.reconciler.common.Constants.COMPONENT_SECRET;
import static com.hedera.solo.operator.reconciler.common.Constants.LABEL_K8S_COMPONENT;
import static com.hedera.solo.operator.reconciler.common.Constants.LABEL_K8S_MANAGED_BY;
import static com.hedera.solo.operator.reconciler.common.Constants.SOLO_OPERATOR;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.reconciler.common.Constants;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.ConfigMapList;
import io.fabric8.kubernetes.api.model.Event;
import io.fabric8.kubernetes.api.model.Pod;
import io.fabric8.kubernetes.api.model.PodList;
import io.fabric8.kubernetes.api.model.Secret;
import io.fabric8.kubernetes.api.model.SecretList;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.fabric8.kubernetes.client.dsl.Resource;
import jakarta.enterprise.context.ApplicationScoped;
import jakarta.inject.Inject;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;

/**
 * It exposes functionalities to retrieve or update resources using the Kubernetes API directly
 *
 * <p>FIXME we could probably refactor this class similar to how the K8 class in solo has been refactored recently
 */
@ApplicationScoped
public class CustomResourceManager {
    private final KubernetesClient client;

    public static CustomResourceManager with(KubernetesClient client) {
        return new CustomResourceManager(client);
    }

    public KubernetesClient getKubernetesClient() {
        return client;
    }

    @Inject
    public CustomResourceManager(KubernetesClient client) {
        this.client = client;
    }

    /**
     * Retrieve a ConfigMap by name.
     *
     * @return ConfigMap if exists, otherwise null
     */
    public ConfigMap configMapByName(String namespace, String name) {
        final List<ConfigMap> list = configMaps(
                        namespace,
                        Map.of(
                                LABEL_K8S_MANAGED_BY, SOLO_OPERATOR,
                                LABEL_K8S_COMPONENT, COMPONENT_CONFIG_MAP))
                .getItems();
        for (ConfigMap cm : list) {
            if (cm.getMetadata().getName().equals(name)) {
                return cm;
            }
        }

        return null;
    }

    /**
     * Retrieve an Orbit by name.
     *
     * @return Orbit if exists, otherwise null
     */
    public Orbit orbitByName(String name) {
        final List<Resource<Orbit>> resources =
                client.resources(Orbit.class).inNamespace(name).resources().toList();
        for (Resource<Orbit> r : resources) {
            if (r.get() != null && r.get().getMetadata().getName().equals(name)) {
                return r.get();
            }
        }

        return null;
    }

    /**
     * Retrieve a secret by name.
     *
     * @return Secret if exists, otherwise null
     */
    public Secret secretByName(String namespace, String name) {
        final List<Secret> list = secrets(
                        namespace,
                        Map.of(
                                LABEL_K8S_MANAGED_BY, SOLO_OPERATOR,
                                LABEL_K8S_COMPONENT, COMPONENT_SECRET))
                .getItems();
        for (Secret s : list) {
            if (s.getMetadata().getName().equals(name)) {
                return s;
            }
        }

        return null;
    }

    public List<ConsensusCapsule> consensusCapsules() {
        return consensusCapsules(client.getNamespace());
    }

    public List<ConsensusCapsule> consensusCapsules(final String namespace) {
        return consensusCapsules(
                namespace,
                Map.of(
                        LABEL_K8S_MANAGED_BY, SOLO_OPERATOR,
                        LABEL_K8S_COMPONENT, COMPONENT_CONSENSUS_NODE));
    }

    public List<ConsensusCapsule> consensusCapsules(final String namespace, Map<String, String> labels) {
        final List<Resource<ConsensusCapsule>> resources = client.resources(ConsensusCapsule.class)
                .inNamespace(namespace)
                .withLabels(labels)
                .resources()
                .toList();

        // resources to PodList
        final List<ConsensusCapsule> list = new ArrayList<>();
        for (Resource<ConsensusCapsule> r : resources) {
            list.add(r.get());
        }

        return list;
    }

    public List<NetworkOperation> networkOperations() {
        return networkOperations(
                client.getNamespace(),
                Map.of(
                        LABEL_K8S_MANAGED_BY, SOLO_OPERATOR,
                        LABEL_K8S_COMPONENT, COMPONENT_CONSENSUS_NODE));
    }

    public List<NetworkOperation> networkOperations(final String namespace) {
        return networkOperations(namespace, Map.of(LABEL_K8S_MANAGED_BY, SOLO_OPERATOR));
    }

    public List<NetworkOperation> networkOperations(final String namespace, Map<String, String> labels) {
        final List<Resource<NetworkOperation>> resources = client.resources(NetworkOperation.class)
                .inNamespace(namespace)
                .withLabels(labels)
                .resources()
                .toList();

        // resources to PodList
        final List<NetworkOperation> list = new ArrayList<>();
        for (Resource<NetworkOperation> r : resources) {
            list.add(r.get());
        }

        return list;
    }

    public ConfigMapList configMaps() {
        return configMaps(client.getNamespace());
    }

    public ConfigMapList configMaps(String namespace) {
        return configMaps(
                namespace,
                Map.of(
                        LABEL_K8S_MANAGED_BY, SOLO_OPERATOR,
                        LABEL_K8S_COMPONENT, COMPONENT_CONFIG_MAP));
    }

    public ConfigMapList configMaps(String namespace, Map<String, String> labels) {
        final List<Resource<ConfigMap>> resources = client.resources(ConfigMap.class)
                .inNamespace(namespace)
                .withLabels(labels)
                .resources()
                .toList();

        // resources to PodList
        final ConfigMapList list = new ConfigMapList();
        for (Resource<ConfigMap> r : resources) {
            list.getItems().add(r.get());
        }

        return list;
    }

    public SecretList secrets() {
        return secrets(client.getNamespace());
    }

    public SecretList secrets(String namespace) {
        return secrets(
                namespace,
                Map.of(
                        LABEL_K8S_MANAGED_BY, SOLO_OPERATOR,
                        LABEL_K8S_COMPONENT, COMPONENT_SECRET));
    }

    public SecretList secrets(String namespace, Map<String, String> labels) {
        final List<Resource<Secret>> resources = client.resources(Secret.class)
                .inNamespace(namespace)
                .withLabels(labels)
                .resources()
                .toList();

        // resources to PodList
        final SecretList list = new SecretList();
        for (Resource<Secret> r : resources) {
            list.getItems().add(r.get());
        }

        return list;
    }

    public PodList pods() {
        return pods(client.getNamespace());
    }

    public PodList pods(final String namespace) {
        return pods(
                namespace, Map.of(LABEL_K8S_MANAGED_BY, SOLO_OPERATOR, LABEL_K8S_COMPONENT, Constants.COMPONENT_POD));
    }

    public PodList pods(final String namespace, final Map<String, String> labels) {
        final List<Resource<Pod>> resources = client.resources(Pod.class)
                .inNamespace(namespace)
                .withLabels(labels)
                .resources()
                .toList();

        // resources to PodList
        final PodList podList = new PodList();
        for (Resource<Pod> r : resources) {
            podList.getItems().add(r.get());
        }

        return podList;
    }

    public Event triggerEvent(final Event event) {
        return client.resource(event).create();
    }
}
