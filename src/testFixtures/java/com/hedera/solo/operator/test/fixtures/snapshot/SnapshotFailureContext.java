// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.test.fixtures.snapshot;

import io.fabric8.kubernetes.client.KubernetesClient;

public interface SnapshotFailureContext {
    Snapshot client(final KubernetesClient client);
}
