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

import com.hedera.solo.operator.model.v1alpha1.HAProxyCapsule;
import io.fabric8.kubernetes.api.model.Service;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ResourceDiscriminator;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.event.ResourceID;
import io.javaoperatorsdk.operator.processing.event.source.SecondaryToPrimaryMapper;
import java.util.Optional;
import java.util.Set;

@KubernetesDependent(resourceDiscriminator = HAProxyConsensusService.class, labelSelector = "app=consensus-node")
public class HAProxyConsensusService extends KubernetesDependentResource<Service, HAProxyCapsule>
        implements SecondaryToPrimaryMapper<Service>, ResourceDiscriminator<Service, HAProxyCapsule> {
    public HAProxyConsensusService() {
        super(Service.class);
    }

    @Override
    public Set<ResourceID> toPrimaryResourceIDs(Service resource) {
        String haproxy = resource.getMetadata().getLabels().get("haproxy");
        return null == haproxy
                ? Set.of()
                : Set.of(new ResourceID(haproxy, resource.getMetadata().getNamespace()));
    }

    @Override
    public Optional<Service> distinguish(
            Class<Service> resource, HAProxyCapsule primary, Context<HAProxyCapsule> context) {
        return context.getSecondaryResources(Service.class).stream()
                .filter(svc -> primary.getMetadata()
                        .getName()
                        .equals(svc.getMetadata().getLabels().get("haproxy")))
                .findAny();
    }
}
