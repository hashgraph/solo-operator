// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.fixtures.snapshot;

import io.fabric8.kubernetes.api.model.Container;
import io.fabric8.kubernetes.api.model.Pod;
import io.fabric8.kubernetes.client.KubernetesClient;
import java.util.function.BiPredicate;

/** The {@code Snapshot} class provides a way to capture and manipulate Kubernetes resources for testing purposes. */
public abstract class Snapshot {
    public static final String[] ANY_NAMESPACE = {null};
    protected final KubernetesClient client;
    protected String[] namespaces;

    /**
     * Constructs a new {@code Snapshot} instance with the provided Kubernetes client.
     *
     * @param client The Kubernetes client used to interact with the cluster.
     */
    protected Snapshot(final KubernetesClient client) {
        this.client = client;
        namespaces();
    }

    /**
     * Sets the namespaces for which resources will be captured or manipulated.
     *
     * @param arrayOfNamespaces An array of namespace strings, or null to capture resources across all namespaces.
     * @return The current {@code Snapshot} instance for method chaining
     */
    public Snapshot namespaces(final String... arrayOfNamespaces) {
        namespaces = arrayOfNamespaces;
        return this;
    }

    /**
     * Captures namespaced Kubernetes resources based on the provided group and version.
     *
     * @param groupVersions An array of strings representing the API group and version for the resources to be captured.
     * @return The current {@code Snapshot} instance for method chaining
     */
    public Snapshot namespacedResources(final String... groupVersions) {
        return resources(NonNamespaced.EXCLUDE, groupVersions);
    }

    /**
     * Captures non-namespaced Kubernetes resources based on the provided group and version.
     *
     * @param groupVersions An array of strings representing the API group and version for the resources to be captured.
     * @return The current {@code Snapshot} instance for method chaining
     */
    public Snapshot nonNamespacedResources(final String... groupVersions) {
        String[] arrayOfNamespaces = namespaces;
        return namespaces().resources(NonNamespaced.INCLUDE, groupVersions).namespaces(arrayOfNamespaces);
    }

    /**
     * Abstract method to capture Kubernetes resources based on the specified criteria.
     *
     * @param nonNamespaced A flag indicating whether to include or exclude namespaced resources.
     * @param groupVersions An array of strings representing the API group and version for the resources to be captured.
     * @return The current {@code Snapshot} instance for method chaining
     */
    public abstract Snapshot resources(NonNamespaced nonNamespaced, String... groupVersions);

    /**
     * Captures logs from all pods and containers that match the default acceptance criteria.
     *
     * @return The current {@code Snapshot} instance for method chaining
     */
    public Snapshot logs() {
        return logs((pod, container) -> true);
    }

    /**
     * Abstract method to capture logs from specific pods and containers that match the provided acceptance criteria.
     *
     * @param accept A {@code BiPredicate} function that defines the acceptance criteria for pods and containers.
     * @return The current {@code Snapshot} instance for method chaining
     */
    public abstract Snapshot logs(BiPredicate<Pod, Container> accept);

    /**
     * Executes a script in a specific pod and container using 'sh -c'.
     *
     * @param name The name of the pod to execute the script in.
     * @param selector A query to select the pods that match the criteria.
     * @param container The name of the container within the pod where the script will be executed.
     * @param script The shell command or script to execute inside the container.
     * @return The current {@code Snapshot} instance for method chaining
     */
    public Snapshot exec(final String name, final String selector, final String container, final String script) {
        return exec(name, selector, container, "sh", "-c", script);
    }

    /**
     * Abstract method to execute a command or script in a specific pod and container.
     *
     * @param name The name of the pod to execute the command or script in.
     * @param selector A query to select the pods that match the criteria.
     * @param container The name of the container within the pod where the command or script will be executed.
     * @param exec An array of strings representing the command and its arguments to be executed inside the container.
     * @return The current {@code Snapshot} instance for method chaining
     */
    public abstract Snapshot exec(String name, String selector, String container, String... exec);

    /**
     * Abstract method to capture files from specific pods and containers.
     *
     * @param selector A query to select the pods that match the criteria.
     * @param container The name of the container within the pod where the file will be captured.
     * @param paths An array of strings representing the paths of the files to be captured inside the container.
     * @return The current {@code Snapshot} instance for method chaining
     */
    public abstract Snapshot file(String selector, String container, String... paths);
}
