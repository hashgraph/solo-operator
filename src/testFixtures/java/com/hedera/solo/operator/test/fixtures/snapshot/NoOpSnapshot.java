// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.fixtures.snapshot;

import io.fabric8.kubernetes.api.model.Container;
import io.fabric8.kubernetes.api.model.Pod;
import io.fabric8.kubernetes.client.KubernetesClient;
import java.util.function.BiPredicate;

public class NoOpSnapshot extends Snapshot {
    protected NoOpSnapshot(final KubernetesClient client) {
        super(client);
    }

    @Override
    public Snapshot resources(final NonNamespaced nonNamespaced, final String... groupVersions) {
        return this;
    }

    @Override
    public Snapshot logs(final BiPredicate<Pod, Container> accept) {
        return this;
    }

    @Override
    public Snapshot exec(final String name, final String selector, final String container, final String... exec) {
        return this;
    }

    @Override
    public Snapshot file(final String selector, final String container, final String... paths) {
        return this;
    }
}
