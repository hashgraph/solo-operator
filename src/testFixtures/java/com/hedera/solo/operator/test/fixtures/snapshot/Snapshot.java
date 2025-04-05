// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.fixtures.snapshot;

import io.fabric8.kubernetes.api.model.Container;
import io.fabric8.kubernetes.api.model.Pod;
import io.fabric8.kubernetes.client.KubernetesClient;
import java.util.function.BiPredicate;

public abstract class Snapshot {
    public static final String[] ANY_NAMESPACE = {null};
    protected final KubernetesClient client;
    protected String[] namespaces;

    protected Snapshot(final KubernetesClient client) {
        this.client = client;
        namespaces();
    }

    public Snapshot namespaces(final String... namespaces) {
        this.namespaces = namespaces;
        return this;
    }

    public Snapshot namespacedResources(final String... groupVersions) {
        return resources(NonNamespaced.EXCLUDE, groupVersions);
    }

    public Snapshot nonNamespacedResources(final String... groupVersions) {
        String[] namespaces = this.namespaces;
        return namespaces().resources(NonNamespaced.INCLUDE, groupVersions).namespaces(namespaces);
    }

    public abstract Snapshot resources(final NonNamespaced nonNamespaced, final String... groupVersions);

    public Snapshot logs() {
        return logs((pod, container) -> true);
    }

    public abstract Snapshot logs(final BiPredicate<Pod, Container> accept);

    public Snapshot exec(final String name, final String selector, final String container, final String script) {
        return exec(name, selector, container, "sh", "-c", script);
    }

    public abstract Snapshot exec(
            final String name, final String selector, final String container, final String... exec);

    public abstract Snapshot file(final String selector, final String container, final String... paths);
}
