// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.fixtures;

import io.fabric8.kubernetes.api.model.HasMetadata;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.fabric8.kubernetes.client.KubernetesClientException;
import java.util.ArrayList;
import java.util.List;
import java.util.Objects;

public final class Finalizers {

    /** Utility class constructor. */
    public Finalizers() {
        throw new UnsupportedOperationException("Utility class");
    }

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

    public static <T extends HasMetadata> List<T> removeFromAll(
            final KubernetesClient client, final Class<T> resourceType) {
        Objects.requireNonNull(client, "client must not be null");
        Objects.requireNonNull(resourceType, "resourceType must not be null");

        final ArrayList<T> updatedResources = new ArrayList<>();

        try {
            client.resources(resourceType).list().getItems().forEach(r -> {
                updatedResources.add(remove(client, r));
            });
        } catch (KubernetesClientException ignored) {
            // ignored exception
        }

        return updatedResources;
    }

    public static <T extends HasMetadata> List<T> removeInNamespace(
            final KubernetesClient client, final String namespace, final Class<T> resourceType) {
        Objects.requireNonNull(client, "client must not be null");
        Objects.requireNonNull(namespace, "namespace must not be null");
        Objects.requireNonNull(resourceType, "resourceType must not be null");

        final ArrayList<T> updatedResources = new ArrayList<>();

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
