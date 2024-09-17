/*
 * Copyright (C) 2024 Hedera Hashgraph, LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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
