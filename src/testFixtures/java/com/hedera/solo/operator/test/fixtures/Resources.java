// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.fixtures;

import static org.awaitility.Awaitility.await;

import io.fabric8.kubernetes.api.model.HasMetadata;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.fabric8.kubernetes.client.KubernetesClientException;
import java.util.Objects;
import java.util.concurrent.TimeUnit;

public final class Resources {

    /** Utility class constructor. */
    private Resources() {
        throw new UnsupportedOperationException("Utility class");
    }

    public static <T extends HasMetadata> void remove(final KubernetesClient client, final T resource) {
        Objects.requireNonNull(client, "client must not be null");
        Objects.requireNonNull(resource, "resource must not be null");

        client.resource(resource).delete();
        await().atMost(30, TimeUnit.SECONDS)
                .until(() -> client.resource(resource).get() == null);
    }

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

    public static <T extends HasMetadata> void cleanup(final KubernetesClient client, final T resource) {
        Objects.requireNonNull(client, "client must not be null");
        Objects.requireNonNull(resource, "resource must not be null");

        client.resource(Finalizers.remove(client, resource)).delete();
        await().atMost(30, TimeUnit.SECONDS)
                .until(() -> client.resource(resource).get() == null);
    }

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
