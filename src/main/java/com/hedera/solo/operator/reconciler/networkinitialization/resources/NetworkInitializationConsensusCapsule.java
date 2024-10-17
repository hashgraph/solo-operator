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

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;

import com.hedera.solo.operator.library.josdk.resource.AbstractKubernetesBulkDependentResource;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitialization;
import com.hedera.solo.operator.reconciler.networkinitialization.NetworkInitializationReconciler;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.Updater;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResourceConfigBuilder;
import io.javaoperatorsdk.operator.processing.event.ResourceID;
import io.javaoperatorsdk.operator.processing.event.source.SecondaryToPrimaryMapper;
import java.util.Map;
import java.util.Set;

@KubernetesDependent
public class NetworkInitializationConsensusCapsule
        extends AbstractKubernetesBulkDependentResource<ConsensusCapsule, NetworkInitialization>
        implements SecondaryToPrimaryMapper<ConsensusCapsule>, Updater<ConsensusCapsule, NetworkInitialization> {
    private final NetworkInitializationReconciler reconciler;

    public NetworkInitializationConsensusCapsule(final NetworkInitializationReconciler reconciler) {
        super(ConsensusCapsule.class);
        this.reconciler = reconciler;
        configureWith(new KubernetesDependentResourceConfigBuilder<ConsensusCapsule>()
                .withUseSSA(Boolean.FALSE)
                .build());
    }

    @Override
    public Map<String, ConsensusCapsule> desiredResources(
            final NetworkInitialization primary, final Context<NetworkInitialization> context) {
        Map<String, ConsensusCapsule> map = getSecondaryResources(primary, context);
        map.replaceAll((k, v) -> edit(v).with(spec -> spec.editSpec().withEligibleForDeployment())
                .build());
        return map;
    }

    @Override
    public Set<ResourceID> toPrimaryResourceIDs(final ConsensusCapsule resource) {
        return reconciler.toPrimaryResourceIDs(resource.getMetadata().getNamespace());
    }
}
