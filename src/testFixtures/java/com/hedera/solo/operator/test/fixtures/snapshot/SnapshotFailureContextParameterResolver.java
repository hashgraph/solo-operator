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

import static com.hedera.solo.operator.test.fixtures.Environment.isSnapshotEnabled;
import static com.hedera.solo.operator.test.fixtures.snapshot.DefaultSnapshot.snapshot;

import io.fabric8.kubernetes.client.KubernetesClient;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.extension.ExtensionContext;
import org.junit.jupiter.api.extension.ParameterContext;
import org.junit.jupiter.api.extension.ParameterResolutionException;
import org.junit.jupiter.api.extension.ParameterResolver;

public class SnapshotFailureContextParameterResolver implements ParameterResolver {
    @Override
    public boolean supportsParameter(final ParameterContext parameterContext, final ExtensionContext extensionContext)
            throws ParameterResolutionException {
        return parameterContext.getParameter().getDeclaringExecutable().isAnnotationPresent(AfterEach.class)
                && parameterContext.getParameter().getType().isAssignableFrom(SnapshotFailureContext.class);
    }

    @Override
    public Object resolveParameter(final ParameterContext parameterContext, final ExtensionContext extensionContext)
            throws ParameterResolutionException {
        if (parameterContext.getParameter().getType().isAssignableFrom(SnapshotFailureContext.class)) {
            return new SnapshotFailureContext() {
                @Override
                public Snapshot client(final KubernetesClient client) {
                    return extensionContext.getExecutionException().isPresent() && isSnapshotEnabled()
                            ? snapshot(extensionContext.getRequiredTestMethod(), client)
                            : new NoOpSnapshot(client);
                }
            };
        }
        return null;
    }
}
