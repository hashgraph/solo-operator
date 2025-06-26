// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.fixtures;

import static org.awaitility.Awaitility.await;

import io.fabric8.kubernetes.api.model.HasMetadata;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.fabric8.kubernetes.client.KubernetesClientException;
import java.util.Objects;
import java.util.concurrent.TimeUnit;

/** Utility class for managing Kubernetes resources. */
public final class Resources {

    /**
     * Private constructor to prevent instantiation of utility class.
     *
     * @throws UnsupportedOperationException if called.
     */
    private Resources() {
        throw new UnsupportedOperationException("Utility class");
    }

    /**
     * Removes a Kubernetes resource by its instance.
     *
     * @param client The Kubernetes client.
     * @param resource The resource to be removed.
     * @param <T> The type of Kubernetes resource that implements {@code HasMetadata}.
     * @throws NullPointerException if the client or resource is null.
     */
    public static <T extends HasMetadata> void remove(final KubernetesClient client, final T resource) {
        Objects.requireNonNull(client, "client must not be null");
        Objects.requireNonNull(resource, "resource must not be null");

        client.resource(resource).delete();
        await().atMost(30, TimeUnit.SECONDS)
                .until(() -> client.resource(resource).get() == null);
    }

    /**
     * Removes all resources of a specific type from the Kubernetes cluster.
     *
     * @param client The Kubernetes client.
     * @param resourceType The class representing the type of resources to be removed.
     * @param <T> The type of Kubernetes resource that implements {@code HasMetadata}.
     * @throws NullPointerException if the client or resourceType is null.
     */
    public static <T extends HasMetadata> void removeAll(final KubernetesClient client, final Class<T> resourceType) {
        Objects.requireNonNull(client, "client must not be null");
        Objects.requireNonNull(resourceType, "resourceType must not be null");

        try {
            client.resources(resourceType).list().getItems().forEach(r -> {
                remove(client, r);
            });
        } catch (KubernetesClientException ignored) {
            // ignored exception
        }
    }

    /**
     * Removes a Kubernetes resource by its name and type.
     *
     * @param client The Kubernetes client.
     * @param resourceType The class representing the type of resource to be removed.
     * @param name The name of the resource to be removed.
     * @param <T> The type of Kubernetes resource that implements {@code HasMetadata}.
     * @throws NullPointerException if the client, resourceType, or name is null.
     */
    public static <T extends HasMetadata> void removeByName(
            final KubernetesClient client, final Class<T> resourceType, final String name) {
        Objects.requireNonNull(client, "client must not be null");
        Objects.requireNonNull(resourceType, "resourceType must not be null");
        Objects.requireNonNull(name, "name must not be null");

        final T resource = client.resources(resourceType).withName(name).get();
        if (resource != null) {
            remove(client, resource);
        }
    }

    /**
     * Cleans up Kubernetes resources by name. This method retrieves a resource of the specified type and name from the
     * Kubernetes cluster using the provided client, then calls {@link #cleanup(KubernetesClient, HasMetadata)} to
     * delete it if found.
     *
     * @param <T> The type of Kubernetes resource that implements {@code HasMetadata}.
     * @param client The Kubernetes client used to interact with the cluster.
     * @param resourceType The class representing the type of Kubernetes resource to retrieve.
     * @param name The name of the Kubernetes resource to be cleaned up.
     * @throws NullPointerException if any of the parameters (client, resourceType, or name) are null.
     */
    public static <T extends HasMetadata> void cleanupByName(
            final KubernetesClient client, final Class<T> resourceType, final String name) {
        Objects.requireNonNull(client, "client must not be null");
        Objects.requireNonNull(resourceType, "resourceType must not be null");
        Objects.requireNonNull(name, "name must not be null");

        final T resource = client.resources(resourceType).withName(name).get();
        if (resource != null) {
            cleanup(client, resource);
        }
    }

    /**
     * Cleans up a Kubernetes resource by its instance.
     *
     * @param client The Kubernetes client.
     * @param resource The resource to be cleaned up.
     * @param <T> The type of Kubernetes resource that implements {@code HasMetadata}.
     * @throws NullPointerException if the client or resource is null.
     */
    public static <T extends HasMetadata> void cleanup(final KubernetesClient client, final T resource) {
        Objects.requireNonNull(client, "client must not be null");
        Objects.requireNonNull(resource, "resource must not be null");

        client.resource(Finalizers.remove(client, resource)).delete();
        await().atMost(30, TimeUnit.SECONDS)
                .until(() -> client.resource(resource).get() == null);
    }

    /**
     * Cleans up all resources of a specific type from the Kubernetes cluster.
     *
     * @param client The Kubernetes client.
     * @param resourceType The class representing the type of resources to be cleaned up.
     * @param <T> The type of Kubernetes resource that implements {@code HasMetadata}.
     * @throws NullPointerException if the client or resourceType is null.
     */
    public static <T extends HasMetadata> void cleanupAll(final KubernetesClient client, final Class<T> resourceType) {
        Objects.requireNonNull(client, "client must not be null");
        Objects.requireNonNull(resourceType, "resourceType must not be null");

        try {
            client.resources(resourceType).list().getItems().forEach(r -> {
                cleanup(client, r);
            });
        } catch (KubernetesClientException ignored) {
            // ignored exception
        }
    }

    /**
     * Cleans up all resources of a specific type in a given namespace.
     *
     * @param client The Kubernetes client.
     * @param namespace The namespace where the resources are located.
     * @param resourceType The class representing the type of resources to be cleaned up.
     * @param <T> The type of Kubernetes resource that implements {@code HasMetadata}.
     * @throws NullPointerException if the client, namespace, or resourceType is null.
     */
    public static <T extends HasMetadata> void cleanupInNamespace(
            final KubernetesClient client, final String namespace, final Class<T> resourceType) {
        Objects.requireNonNull(client, "client must not be null");
        Objects.requireNonNull(namespace, "namespace must not be null");
        Objects.requireNonNull(resourceType, "resourceType must not be null");

        try {
            client.resources(resourceType)
                    .inNamespace(namespace)
                    .list()
                    .getItems()
                    .forEach(r -> {
                        cleanup(client, r);
                    });
        } catch (KubernetesClientException ignored) {
            // ignored exception
        }
    }
}
