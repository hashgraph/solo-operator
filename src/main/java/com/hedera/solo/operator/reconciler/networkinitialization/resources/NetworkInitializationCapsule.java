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

package com.hedera.solo.operator.reconciler.networkinitialization.resources;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitialization;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.BulkDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.Updater;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.BooleanWithUndefined;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.event.ResourceID;
import io.javaoperatorsdk.operator.processing.event.source.SecondaryToPrimaryMapper;
import java.util.Map;
import java.util.Set;
import java.util.function.Function;
import java.util.stream.Collectors;

// why do I need SSA=false here, but not in ConsensusCapsuleReconciler?
@KubernetesDependent(useSSA = BooleanWithUndefined.FALSE)
public class NetworkInitializationCapsule extends KubernetesDependentResource<ConsensusCapsule, NetworkInitialization>
        implements BulkDependentResource<ConsensusCapsule, NetworkInitialization>,
                SecondaryToPrimaryMapper<ConsensusCapsule>,
                Updater<ConsensusCapsule, NetworkInitialization> {
    public NetworkInitializationCapsule() {
        super(ConsensusCapsule.class);
    }

    @Override
    public Map<String, ConsensusCapsule> desiredResources(
            NetworkInitialization primary, Context<NetworkInitialization> context) {
        return getSecondaryResources(primary, context).entrySet().stream()
                .collect(Collectors.toMap(Map.Entry::getKey, v -> v.getValue()
                        .edit()
                        .editSpec()
                        .withEligibleForDeployment(true)
                        .endSpec()
                        .build()));
    }

    @Override
    public Map<String, ConsensusCapsule> getSecondaryResources(
            NetworkInitialization primary, Context<NetworkInitialization> context) {
        return context.getSecondaryResourcesAsStream(ConsensusCapsule.class)
                .collect(Collectors.toMap($ -> $.getMetadata().getName(), Function.identity()));
    }

    @Override
    public Set<ResourceID> toPrimaryResourceIDs(ConsensusCapsule resource) {
        return Set.of(
                new ResourceID("network-initialization", resource.getMetadata().getNamespace()));
    }
}
