// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.fixtures.snapshot;

import static com.hedera.solo.operator.library.generic.Iterables.asList;
import static com.hedera.solo.operator.test.fixtures.snapshot.CallStack.calledFrom;

import io.fabric8.kubernetes.api.model.APIGroup;
import io.fabric8.kubernetes.api.model.APIResource;
import io.fabric8.kubernetes.api.model.Container;
import io.fabric8.kubernetes.api.model.GenericKubernetesResource;
import io.fabric8.kubernetes.api.model.GroupVersionForDiscovery;
import io.fabric8.kubernetes.api.model.Pod;
import io.fabric8.kubernetes.api.model.PodSpec;
import io.fabric8.kubernetes.client.KubernetesClient;
import io.fabric8.kubernetes.client.KubernetesClientException;
import io.fabric8.kubernetes.client.dsl.AnyNamespaceOperation;
import io.fabric8.kubernetes.client.dsl.BytesLimitTerminateTimeTailPrettyLoggable;
import io.fabric8.kubernetes.client.dsl.ContainerResource;
import io.fabric8.kubernetes.client.dsl.ExecWatch;
import io.fabric8.kubernetes.client.dsl.MixedOperation;
import io.fabric8.kubernetes.client.dsl.PodResource;
import io.fabric8.kubernetes.client.dsl.Resource;
import java.io.ByteArrayOutputStream;
import java.lang.reflect.Method;
import java.util.ArrayList;
import java.util.Collection;
import java.util.HashSet;
import java.util.List;
import java.util.Set;
import java.util.function.BiPredicate;
import java.util.function.Function;

/** The DefaultSnapshot class provides methods to capture Kubernetes resources and logs for debugging purposes. */
public class DefaultSnapshot extends Snapshot {
    private final SnapshotWriter writer;

    /**
     * Constructs a new DefaultSnapshot instance with the given writer and Kubernetes client.
     *
     * @param writer The writer used to save captured resources and logs.
     * @param client The Kubernetes client for interacting with the cluster.
     */
    public DefaultSnapshot(final SnapshotWriter writer, final KubernetesClient client) {
        super(client);
        this.writer = writer;
    }

    /**
     * Creates a new DefaultSnapshot instance using the default snapshot writer and caller method information.
     *
     * @param client The Kubernetes client for interacting with the cluster.
     * @return A new DefaultSnapshot instance
     */
    public static DefaultSnapshot snapshot(final KubernetesClient client) {
        return new DefaultSnapshot(new DefaultSnapshotWriter(calledFrom()), client);
    }

    /**
     * Creates a new DefaultSnapshot instance using the provided method and caller information.
     *
     * @param method The method from which this call is made, used for logging purposes.
     * @param client The Kubernetes client for interacting with the cluster.
     * @return A new DefaultSnapshot instance
     */
    public static DefaultSnapshot snapshot(final Method method, final KubernetesClient client) {
        return new DefaultSnapshot(new DefaultSnapshotWriter(method), client);
    }

    @Override
    public DefaultSnapshot resources(final NonNamespaced nonNamespaced, final String... groupVersions) {
        Set<APIResource> seen = new HashSet<>();
        for (String groupVersion : 0 == groupVersions.length ? groupVersions() : groupVersions) {
            try {
                for (APIResource resource : client.getApiResources(groupVersion).getResources()) {
                    if (!seen.contains(resource) && resource.getVerbs().contains("list")) {
                        if (resource.getNamespaced()) {
                            for (String namespace : namespaces) {
                                resource(groupVersion, resource, namespace);
                            }
                        } else if (nonNamespaced.include) {
                            resource(groupVersion, resource, null);
                        }
                        seen.add(resource);
                    }
                }
            } catch (KubernetesClientException ignored) {
                // Ignore exceptions for group versions that cannot be accessed
            }
        }
        return this;
    }

    @Override
    public DefaultSnapshot logs(final BiPredicate<Pod, Container> accept) {
        for (String namespace : namespaces) {
            try {
                for (Pod pod : inNamespace(client.pods(), namespace).list().getItems()) {
                    PodSpec podSpec = pod.getSpec();
                    for (Container container : asList(podSpec.getInitContainers(), podSpec.getContainers())) {
                        logs(accept, pod, container);
                    }
                }
            } catch (KubernetesClientException ignored) {
                // Ignore exceptions for pods that cannot be accessed
            }
        }
        return this;
    }

    @Override
    public DefaultSnapshot exec(
            final String name, final String selector, final String container, final String... exec) {
        for (String namespace : namespaces) {
            try {
                for (Pod pod : inNamespace(client.pods(), namespace)
                        .withLabelSelector(selector)
                        .list()
                        .getItems()) {
                    try {
                        PodResource resource = podResource(pod);
                        ByteArrayOutputStream out = new ByteArrayOutputStream();
                        ByteArrayOutputStream err = new ByteArrayOutputStream();
                        try (ExecWatch watch = resource.inContainer(container)
                                .writingOutput(out)
                                .writingError(err)
                                .exec(exec)) {
                            ExecResult result =
                                    new ExecResult(exec, watch.exitCode().join(), out.toString(), err.toString());
                            writer.write(
                                    result, "exec", namespace, pod.getMetadata().getName(), container, name);
                        }
                    } catch (KubernetesClientException ignored) {
                        // Ignore exceptions for pods that cannot be accessed
                    }
                }
            } catch (KubernetesClientException ignored) {
                // Ignore exceptions for pods that cannot be accessed
            }
        }
        return this;
    }

    @Override
    public DefaultSnapshot file(final String selector, final String container, final String... paths) {
        for (String namespace : namespaces) {
            try {
                for (Pod pod : inNamespace(client.pods(), namespace)
                        .withLabelSelector(selector)
                        .list()
                        .getItems()) {
                    try {
                        ContainerResource resource = podResource(pod).inContainer(container);
                        for (String path : paths) {
                            try {
                                resource.file(path)
                                        .copy(writer.path(
                                                "file",
                                                namespace,
                                                pod.getMetadata().getName(),
                                                container,
                                                path));
                            } catch (KubernetesClientException ignored) {
                                // Ignore exceptions for files that cannot be copied
                            }
                        }
                    } catch (KubernetesClientException ignored) {
                        // Ignore exceptions for pods that cannot be accessed
                    }
                }
            } catch (KubernetesClientException ignored) {
                // Ignore exceptions for pods that cannot be accessed
            }
        }
        return this;
    }

    /**
     * Logs the output of a container from a pod that matches the accept predicate.
     *
     * @param accept A predicate to determine which pods and containers' logs to capture.
     * @param pod The Kubernetes Pod object.
     * @param container The Kubernetes Container object.
     */
    private void logs(final BiPredicate<Pod, Container> accept, final Pod pod, final Container container) {
        if (!accept.test(pod, container)) {
            return;
        }
        try {
            BytesLimitTerminateTimeTailPrettyLoggable loggable =
                    podResource(pod).inContainer(container.getName()).usingTimestamps();
            String namespace = pod.getMetadata().getNamespace();
            String podName = pod.getMetadata().getName();
            String containerName = container.getName();
            writer.write(loggable.getLog(), "logs", namespace, podName, containerName, "live");
            writer.write(loggable.terminated().getLog(), "logs", namespace, podName, containerName, "dead");
        } catch (KubernetesClientException ignored) {
            // Ignore exceptions for pods or containers that cannot be accessed or logged
        }
    }

    /**
     * Retrieves a PodResource object for the given pod.
     *
     * @param pod The Kubernetes Pod object.
     * @return A PodResource object for the specified pod
     */
    private PodResource podResource(final Pod pod) {
        return (PodResource) client.pods()
                .inNamespace(pod.getMetadata().getNamespace())
                .withName(pod.getMetadata().getName())
                .usingListener(((code, reason) -> {}));
    }

    /**
     * Retrieves the list of supported group versions from the Kubernetes cluster.
     *
     * @return An array of group version strings
     */
    private String[] groupVersions() {
        List<String> groupVersions = new ArrayList<>(List.of("v1"));
        try {
            client.getApiGroups().getGroups().stream()
                    .map(APIGroup::getVersions)
                    .flatMap(Collection::stream)
                    .map(GroupVersionForDiscovery::getGroupVersion)
                    .forEach(groupVersions::add);
        } catch (KubernetesClientException ignored) {
            // If the API groups cannot be retrieved, we ignore the exception.
        }
        return groupVersions.toArray(new String[0]);
    }

    /**
     * Retrieves a generic Kubernetes resource list from the specified namespace.
     *
     * @param client The Kubernetes client for interacting with the cluster.
     * @param namespace The namespace in which to list the resources (or null for non-namespaced resources).
     * @param <T> The type of the resource.
     * @param <L> The type of the list operation.
     * @param <R> The type of the resource operation.
     * @return A list of GenericKubernetesResource objects
     */
    private static <T, L, R extends Resource<T>> AnyNamespaceOperation<T, L, R> inNamespace(
            final MixedOperation<T, L, R> client, final String namespace) {
        return null == namespace ? client.inAnyNamespace() : client.inNamespace(namespace);
    }

    /**
     * A helper method to retrieve a generic Kubernetes resource list from the specified API group version and kind,
     * with optional namespacing.
     *
     * @param client The Kubernetes client for interacting with the cluster.
     * @param function A function to apply to the MixedOperation for retrieving resources.
     * @param <T> The type of the resource.
     * @param <L> The type of the list operation.
     * @param <R> The type of the resource operation.
     * @return An AnyNamespaceOperation object based on the applied function
     */
    private static <T, L, R extends Resource<T>> AnyNamespaceOperation<T, L, R> inNamespace(
            final MixedOperation<T, L, R> client,
            final Function<MixedOperation<T, L, R>, AnyNamespaceOperation<T, L, R>> function) {
        return function.apply(client);
    }

    /**
     * Captures and writes a Kubernetes resource to the writer based on its group version, kind, namespace, and name.
     *
     * @param groupVersion The API group version of the resource.
     * @param resource The Kubernetes APIResource object representing the type of resource to list.
     * @param namespace The namespace in which the resource resides (or null for non-namespaced resources).
     */
    private void resource(final String groupVersion, final APIResource resource, final String namespace) {
        try {
            List<GenericKubernetesResource> list = inNamespace(
                            client.genericKubernetesResources(groupVersion, resource.getKind()),
                            c -> !resource.getNamespaced()
                                    ? c
                                    : null == namespace ? c.inAnyNamespace() : c.inNamespace(namespace))
                    .list()
                    .getItems();
            for (GenericKubernetesResource object : list) {
                writer.write(
                        object,
                        resource.getNamespaced() ? "namespaced" : "non-namespaced",
                        object.getMetadata().getNamespace(),
                        resource.getName(),
                        object.getMetadata().getName());
            }
        } catch (KubernetesClientException ignored) {
            // Ignore exceptions for resources that cannot be listed or accessed
        }
    }
}
