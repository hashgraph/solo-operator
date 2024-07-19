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
import io.fabric8.kubernetes.api.model.Pod;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.event.ResourceID;
import io.javaoperatorsdk.operator.processing.event.source.SecondaryToPrimaryMapper;
import java.util.Collections;
import java.util.Set;

@KubernetesDependent
public class CapsulePod extends KubernetesDependentResource<Pod, ConsensusCapsule>
        implements SecondaryToPrimaryMapper<Pod> {
    public CapsulePod() {
        super(Pod.class);
    }

    @Override
    public Set<ResourceID> toPrimaryResourceIDs(Pod resource) {
        String name = resource.getMetadata().getLabels().get("consensus-capsule");
        return null == name
                ? Collections.emptySet()
                : Set.of(new ResourceID(name, resource.getMetadata().getNamespace()));
    }
}
