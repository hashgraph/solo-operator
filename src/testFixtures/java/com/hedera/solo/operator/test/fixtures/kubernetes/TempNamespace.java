// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.fixtures.kubernetes;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static org.awaitility.Awaitility.await;

import io.fabric8.kubernetes.api.model.Namespace;
import io.fabric8.kubernetes.client.KubernetesClient;
import java.time.Duration;

/**
 * The {@code TempNamespace} class provides a mechanism to create and manage a temporary Kubernetes namespace. It
 * implements {@link AutoCloseable} to ensure that the namespace is deleted when no longer needed.
 */
public class TempNamespace implements AutoCloseable {
    private KubernetesClient kubernetesClient;
    private String name;

    /**
     * Constructs a temporary namespace with a name generated from the class name.
     *
     * @param kubernetesClient The Kubernetes client to interact with the cluster.
     * @param aClass The class whose simple name will be used as the prefix for the namespace name.
     */
    public TempNamespace(final KubernetesClient kubernetesClient, final Class<?> aClass) {
        this(kubernetesClient, aClass.getSimpleName().toLowerCase() + "-");
    }

    /**
     * Constructs a temporary namespace with a name generated from the provided prefix.
     *
     * @param kubernetesClient The Kubernetes client to interact with the cluster.
     * @param prefix The prefix for the namespace name, which will be used to generate a unique name.
     */
    public TempNamespace(final KubernetesClient kubernetesClient, final String prefix) {
        this.kubernetesClient = kubernetesClient;
        this.name = kubernetesClient
                .resource(edit(new Namespace())
                        .with(ns -> ns.editMetadata().withGenerateName(prefix))
                        .build())
                .create()
                .getMetadata()
                .getName();
    }

    /**
     * Returns the name of the temporary namespace.
     *
     * @return the name of the namespace
     */
    public String name() {
        return name;
    }

    @Override
    public void close() {
        kubernetesClient.namespaces().withName(name).delete();
        await().atMost(Duration.ofSeconds(30))
                .until(() -> kubernetesClient.namespaces().withName(name).get() == null);
    }
}
