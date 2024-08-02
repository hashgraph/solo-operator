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

package com.hedera.solo.operator.reconciler.capsule.haproxy.resources;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.HAProxyCapsule;
import com.hedera.solo.operator.model.v1alpha1.consensuscapsulespec.DependentResourceLabels;
import io.javaoperatorsdk.operator.api.config.informer.InformerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.Updater;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.BooleanWithUndefined;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.event.ResourceID;
import io.javaoperatorsdk.operator.processing.event.source.SecondaryToPrimaryMapper;
import java.util.Set;

@KubernetesDependent(useSSA = BooleanWithUndefined.FALSE)
public class HAProxyConsensusCapsule extends KubernetesDependentResource<ConsensusCapsule, HAProxyCapsule>
        implements Updater<ConsensusCapsule, HAProxyCapsule>, SecondaryToPrimaryMapper<ConsensusCapsule> {
    public HAProxyConsensusCapsule() {
        super(ConsensusCapsule.class);
    }

    @Override
    protected ConsensusCapsule desired(HAProxyCapsule primary, Context<HAProxyCapsule> context) {
        return context.getSecondaryResource(ConsensusCapsule.class)
                .map(cc -> cc.edit()
                        .editMetadata()
                        .endMetadata()
                        .editSpec()
                        .removeMatchingFromDependentResourceLabels(e -> "haproxy".equals(e.getName()))
                        .addNewDependentResourceLabel()
                        .withName("haproxy")
                        .withValue(primary.getMetadata().getName())
                        .endDependentResourceLabel()
                        .endSpec()
                        .build())
                .orElse(null);
    }

    @Override
    public Set<ResourceID> toPrimaryResourceIDs(ConsensusCapsule resource) {
        String haproxy = resource.getSpec().getDependentResourceLabels().stream()
                .filter(labels -> "haproxy".equals(labels.getName()))
                .findAny()
                .map(DependentResourceLabels::getValue)
                .orElse(null);
        return null == haproxy
                ? Set.of()
                : Set.of(new ResourceID(haproxy, resource.getMetadata().getNamespace()));
    }

    @Override
    protected InformerConfiguration.InformerConfigurationBuilder<ConsensusCapsule> informerConfigurationBuilder() {
        return super.informerConfigurationBuilder()
                .withPrimaryToSecondaryMapper(p -> Set.of(new ResourceID(
                        ((HAProxyCapsule) p).getSpec().getConsensus().getNode(),
                        p.getMetadata().getNamespace())));
    }
}
