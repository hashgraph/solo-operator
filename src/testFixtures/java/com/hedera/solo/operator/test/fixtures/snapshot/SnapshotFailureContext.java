// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.fixtures.snapshot;

import io.fabric8.kubernetes.client.KubernetesClient;

/** The SnapshotFailureContext interface defines a method for obtaining a Kubernetes client and creating a snapshot. */
public interface SnapshotFailureContext {
    /**
     * Method to create a snapshot using the provided Kubernetes client.
     *
     * @param client The Kubernetes client used to interact with the cluster.
     * @return A snapshot of the current state of the Kubernetes resources
     */
    Snapshot client(KubernetesClient client);
}
