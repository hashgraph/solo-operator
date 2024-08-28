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

package com.hedera.solo.operator.reconciler.orbit.resources;

import static com.hedera.solo.operator.reconciler.capsule.consensus.ConsensusCapsuleConstants.LABEL_SELECTOR_CONSENSUS_NODE;
import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;

import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.reconciler.common.Metadata;
import io.fabric8.kubernetes.api.model.Service;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ResourceDiscriminator;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDNoGCKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import java.util.Optional;

@KubernetesDependent(
        labelSelector = LABEL_SELECTOR_CONSENSUS_NODE,
        resourceDiscriminator = ConsensusHeadlessService.class)
public class ConsensusHeadlessService extends CRUDNoGCKubernetesDependentResource<Service, Orbit>
        implements ResourceDiscriminator<Service, Orbit> {
    public static final String TEMPLATE = "headless-service.yaml";

    public ConsensusHeadlessService() {
        super(Service.class);
    }

    @Override
    protected Service desired(Orbit primary, Context<Orbit> context) {
        return Metadata.applyOrbitMetaTo(
                        primary,
                        loadYaml(Service.class, getClass(), TEMPLATE).edit().editMetadata())
                .withNamespace(primary.getMetadata().getName())
                .endMetadata()
                .build();
    }

    @Override
    public Optional<Service> distinguish(Class<Service> resource, Orbit primary, Context<Orbit> context) {
        return context.getSecondaryResources(Service.class).stream()
                .filter(service -> "None".equalsIgnoreCase(service.getSpec().getClusterIP()))
                .findAny();
    }
}
