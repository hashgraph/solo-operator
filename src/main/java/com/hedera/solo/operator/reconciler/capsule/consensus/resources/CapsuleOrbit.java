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

package com.hedera.solo.operator.reconciler.capsule.consensus.resources;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.Orbit;
import io.javaoperatorsdk.operator.api.config.informer.InformerConfiguration;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.event.ResourceID;
import io.javaoperatorsdk.operator.processing.event.source.SecondaryToPrimaryMapper;
import java.util.Set;

@KubernetesDependent
public class CapsuleOrbit extends KubernetesDependentResource<Orbit, ConsensusCapsule>
        implements SecondaryToPrimaryMapper<Orbit> {
    public CapsuleOrbit() {
        super(Orbit.class);
    }

    @Override
    protected InformerConfiguration.InformerConfigurationBuilder<Orbit> informerConfigurationBuilder() {
        return super.informerConfigurationBuilder()
                .withPrimaryToSecondaryMapper(
                        p -> Set.of(new ResourceID(p.getMetadata().getNamespace())));
    }

    @Override
    public Set<ResourceID> toPrimaryResourceIDs(Orbit resource) {
        return Set.of(
                //                new ResourceID("node0", resource.getMetadata().getName()),
                //                new ResourceID("node1", resource.getMetadata().getName()),
                //                new ResourceID("node2", resource.getMetadata().getName())
                );
    }
}
