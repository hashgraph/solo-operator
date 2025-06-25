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
import java.util.Objects;

/**
 * It exposes functionalities to retrieve or update resources using the Kubernetes API directly.
 *
 * <p>FIXME we could probably refactor this class similar to how the K8 class in solo has been refactored recently
 */
@ApplicationScoped
public class CustomResourceManager {
    private final KubernetesClient client;

    /**
     * Factory method to create an instance of CustomResourceManager with the provided Kubernetes client.
     *
     * @param client The Kubernetes client to be used for resource management
     * @return A new instance of CustomResourceManager
     */
    public static CustomResourceManager with(KubernetesClient client) {
        return new CustomResourceManager(client);
    }

    /**
     * Get the Kubernetes client.
     *
     * @return The Kubernetes client instance
     */
    public KubernetesClient getKubernetesClient() {
        return client;
    }

    /**
     * Constructor for CustomResourceManager.
     *
     * @param client The Kubernetes client to be used for resource management
     */
    @Inject
    public CustomResourceManager(KubernetesClient client) {
        this.client = client;
    }

    /**
     * Retrieve a ConfigMap by its namespace and name.
     *
     * @param namespace The Kubernetes namespace where the ConfigMap is located.
     * @param name The name of the ConfigMap to retrieve.
     * @return The retrieved ConfigMap, or null if it does not exist
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
     * Retrieve an Orbit by its name.
     *
     * @param name The name of the Orbit to retrieve.
     * @return The retrieved Orbit, or null if it does not exist
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
     * Retrieve a secret by its namespace and name.
     *
     * @param namespace The Kubernetes namespace where the secret is located.
     * @param name The name of the secret to retrieve.
     * @return The retrieved secret, or null if it does not exist
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

    /**
     * Retrieve all consensus capsules in the default namespace.
     *
     * @return A list of ConsensusCapsule objects
     */
    public List<ConsensusCapsule> consensusCapsules() {
        return consensusCapsules(client.getNamespace());
    }

    /**
     * Retrieve all consensus capsules in a specified namespace.
     *
     * @param namespace The Kubernetes namespace to search for ConsensusCapsule objects.
     * @return A list of ConsensusCapsule objects
     */
    public List<ConsensusCapsule> consensusCapsules(final String namespace) {
        return consensusCapsules(
                namespace,
                Map.of(
                        LABEL_K8S_MANAGED_BY, SOLO_OPERATOR,
                        LABEL_K8S_COMPONENT, COMPONENT_CONSENSUS_NODE));
    }

    /**
     * Retrieve all consensus capsules in a specified namespace and with specific labels.
     *
     * @param namespace The Kubernetes namespace to search for ConsensusCapsule objects.
     * @param labels A map of labels used to filter the search.
     * @return A list of ConsensusCapsule objects
     */
    public List<ConsensusCapsule> consensusCapsules(final String namespace, Map<String, String> labels) {
        final List<Resource<ConsensusCapsule>> resources = client.resources(ConsensusCapsule.class)
                .inNamespace(namespace)
                .withLabels(labels)
                .resources()
                .toList();

        // Convert resources to a list of ConsensusCapsule objects
        final List<ConsensusCapsule> list = new ArrayList<>();
        for (Resource<ConsensusCapsule> r : resources) {
            list.add(r.get());
        }

        return list;
    }

    /**
     * Retrieve all network operations in the default namespace.
     *
     * @return A list of NetworkOperation objects
     */
    public List<NetworkOperation> networkOperations() {
        return networkOperations(null);
    }

    /**
     * Retrieve all network operations in a specified namespace.
     *
     * @param namespace The Kubernetes namespace to search for NetworkOperation objects.
     * @return A list of NetworkOperation objects
     */
    public List<NetworkOperation> networkOperations(final String namespace) {
        return networkOperations(namespace, Map.of(LABEL_K8S_MANAGED_BY, SOLO_OPERATOR));
    }

    /**
     * Retrieve all network operations in a specified namespace and with specific labels.
     *
     * @param namespace The Kubernetes namespace to search for NetworkOperation objects.
     * @param labels A map of labels used to filter the search.
     * @return A list of NetworkOperation objects
     */
    public List<NetworkOperation> networkOperations(final String namespace, Map<String, String> labels) {
        final List<Resource<NetworkOperation>> resources;
        if (namespace == null) {
            resources = client.resources(NetworkOperation.class)
                    .inAnyNamespace()
                    .withLabels(labels)
                    .resources()
                    .toList();
        } else {
            resources = client.resources(NetworkOperation.class)
                    .inNamespace(namespace)
                    .withLabels(labels)
                    .resources()
                    .toList();
        }

        // Convert resources to a list of NetworkOperation objects
        final List<NetworkOperation> list = new ArrayList<>();
        for (Resource<NetworkOperation> r : resources) {
            list.add(r.get());
        }

        list.removeIf(Objects::isNull);

        return list;
    }

    /**
     * Retrieve all ConfigMap objects in the default namespace.
     *
     * @return A ConfigMapList containing all retrieved ConfigMaps
     */
    public ConfigMapList configMaps() {
        return configMaps(client.getNamespace());
    }

    /**
     * Retrieve all ConfigMap objects in a specified namespace.
     *
     * @param namespace The Kubernetes namespace to search for ConfigMap objects.
     * @return A ConfigMapList containing all retrieved ConfigMaps
     */
    public ConfigMapList configMaps(String namespace) {
        return configMaps(
                namespace,
                Map.of(
                        LABEL_K8S_MANAGED_BY, SOLO_OPERATOR,
                        LABEL_K8S_COMPONENT, COMPONENT_CONFIG_MAP));
    }

    /**
     * Retrieve all ConfigMap objects in a specified namespace and with specific labels.
     *
     * @param namespace The Kubernetes namespace to search for ConfigMap objects.
     * @param labels A map of labels used to filter the search.
     * @return A ConfigMapList containing all retrieved ConfigMaps
     */
    public ConfigMapList configMaps(String namespace, Map<String, String> labels) {
        final List<Resource<ConfigMap>> resources = client.resources(ConfigMap.class)
                .inNamespace(namespace)
                .withLabels(labels)
                .resources()
                .toList();

        // Convert resources to a ConfigMapList
        final ConfigMapList list = new ConfigMapList();
        for (Resource<ConfigMap> r : resources) {
            list.getItems().add(r.get());
        }

        return list;
    }

    /**
     * Retrieve all secret objects in the default namespace.
     *
     * @return A SecretList containing all retrieved secrets
     */
    public SecretList secrets() {
        return secrets(client.getNamespace());
    }

    /**
     * Retrieve all secret objects in a specified namespace.
     *
     * @param namespace The Kubernetes namespace to search for secret objects.
     * @return A SecretList containing all retrieved secrets
     */
    public SecretList secrets(String namespace) {
        return secrets(
                namespace,
                Map.of(
                        LABEL_K8S_MANAGED_BY, SOLO_OPERATOR,
                        LABEL_K8S_COMPONENT, COMPONENT_SECRET));
    }

    /**
     * Retrieve all secret objects in a specified namespace and with specific labels.
     *
     * @param namespace The Kubernetes namespace to search for secret objects.
     * @param labels A map of labels used to filter the search.
     * @return A SecretList containing all retrieved secrets
     */
    public SecretList secrets(String namespace, Map<String, String> labels) {
        final List<Resource<Secret>> resources = client.resources(Secret.class)
                .inNamespace(namespace)
                .withLabels(labels)
                .resources()
                .toList();

        // Convert resources to a SecretList
        final SecretList list = new SecretList();
        for (Resource<Secret> r : resources) {
            list.getItems().add(r.get());
        }

        return list;
    }

    /**
     * Retrieve all pod objects in the default namespace.
     *
     * @return A PodList containing all retrieved pods
     */
    public PodList pods() {
        return pods(client.getNamespace());
    }

    /**
     * Retrieve all pod objects in a specified namespace.
     *
     * @param namespace The Kubernetes namespace to search for pod objects.
     * @return A PodList containing all retrieved pods
     */
    public PodList pods(final String namespace) {
        return pods(
                namespace,
                Map.of(LABEL_K8S_MANAGED_BY, SOLO_OPERATOR, LABEL_K8S_COMPONENT, Constants.COMPONENT_DEPLOYMENT));
    }

    /**
     * Retrieve all pod objects in a specified namespace and with specific labels.
     *
     * @param namespace The Kubernetes namespace to search for pod objects.
     * @param labels A map of labels used to filter the search.
     * @return A PodList containing all retrieved pods
     */
    public PodList pods(final String namespace, final Map<String, String> labels) {
        final List<Resource<Pod>> resources = client.resources(Pod.class)
                .inNamespace(namespace)
                .withLabels(labels)
                .resources()
                .toList();

        // Convert resources to a PodList
        final PodList podList = new PodList();
        for (Resource<Pod> r : resources) {
            podList.getItems().add(r.get());
        }

        return podList;
    }

    /**
     * Trigger an event in the Kubernetes cluster.
     *
     * @param event The Event object to be created.
     * @return The created Event object
     */
    public Event triggerEvent(final Event event) {
        return client.resource(event).create();
    }
}
