// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.fixtures;

import io.fabric8.kubernetes.api.model.HasMetadata;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.fabric8.kubernetes.client.KubernetesClientException;
import java.util.ArrayList;
import java.util.List;
import java.util.Objects;

/** Utility class to manage finalizers in Kubernetes resources. */
public final class Finalizers {
    /**
     * Private constructor to prevent instantiation of utility class.
     *
     * @throws UnsupportedOperationException if called.
     */
    private Finalizers() {
        throw new UnsupportedOperationException("Utility class");
    }

    /**
     * Removes a Kubernetes resource by name.
     *
     * @param client The Kubernetes client. Must not be null.
     * @param resourceType The type of the Kubernetes resource. Must not be null.
     * @param name The name of the resource to remove. Must not be null.
     * @param <T> The type of the Kubernetes resource.
     * @return The removed resource, or null if the resource was not found
     */
    public static <T extends HasMetadata> T removeByName(
            final KubernetesClient client, final Class<T> resourceType, final String name) {
        Objects.requireNonNull(client, "client must not be null");
        Objects.requireNonNull(resourceType, "resourceType must not be null");
        Objects.requireNonNull(name, "name must not be null");

        final T resource = client.resources(resourceType).withName(name).get();
        if (resource != null) {
            return remove(client, resource);
        }

        return null;
    }

    /**
     * Removes a Kubernetes resource.
     *
     * @param client The Kubernetes client. Must not be null.
     * @param resource The Kubernetes resource to remove. Must not be null.
     * @param <T> The type of the Kubernetes resource.
     * @return The removed resource
     */
    public static <T extends HasMetadata> T remove(final KubernetesClient client, final T resource) {
        Objects.requireNonNull(client, "client must not be null");
        Objects.requireNonNull(resource, "resource must not be null");

        final List<String> finalizers = resource.getMetadata().getFinalizers();
        if (finalizers != null && !finalizers.isEmpty()) {
            resource.getMetadata().setFinalizers(null);
            return client.resource(resource).patch();
        }

        return resource;
    }

    /**
     * Removes all resources of a given type from all namespaces.
     *
     * @param client The Kubernetes client. Must not be null.
     * @param resourceType The type of the Kubernetes resources to remove. Must not be null.
     * @param <T> The type of the Kubernetes resources.
     * @return A list of updated resources
     */
    public static <T extends HasMetadata> List<T> removeFromAll(
            final KubernetesClient client, final Class<T> resourceType) {
        Objects.requireNonNull(client, "client must not be null");
        Objects.requireNonNull(resourceType, "resourceType must not be null");

        final List<T> updatedResources = new ArrayList<>();

        try {
            client.resources(resourceType).list().getItems().forEach(r -> {
                updatedResources.add(remove(client, r));
            });
        } catch (KubernetesClientException ignored) {
            // ignored exception
        }

        return updatedResources;
    }

    /**
     * Removes all resources of a given type from a specific namespace.
     *
     * @param client The Kubernetes client. Must not be null.
     * @param namespace The namespace to remove resources from. Must not be null.
     * @param resourceType The type of the Kubernetes resources to remove. Must not be null.
     * @param <T> The type of the Kubernetes resources.
     * @return A list of updated resources
     */
    public static <T extends HasMetadata> List<T> removeInNamespace(
            final KubernetesClient client, final String namespace, final Class<T> resourceType) {
        Objects.requireNonNull(client, "client must not be null");
        Objects.requireNonNull(namespace, "namespace must not be null");
        Objects.requireNonNull(resourceType, "resourceType must not be null");

        final List<T> updatedResources = new ArrayList<>();

        try {
            client.resources(resourceType)
                    .inNamespace(namespace)
                    .list()
                    .getItems()
                    .forEach(r -> {
                        updatedResources.add(remove(client, r));
                    });
        } catch (KubernetesClientException ignored) {
            // ignored exception
        }

        return updatedResources;
    }
}
