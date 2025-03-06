// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.capsule.consensus.resources;

import static com.hedera.solo.operator.reconciler.common.Constants.LABEL_NODE_NAME;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import io.fabric8.kubernetes.api.model.Pod;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.event.ResourceID;
import io.javaoperatorsdk.operator.processing.event.source.SecondaryToPrimaryMapper;
import java.util.Collections;
import java.util.Set;

@KubernetesDependent(labelSelector = "app.kubernetes.io/managed-by=solo-operator,app.kubernetes.io/part-of=consensus-node,app.kubernetes.io/component=deployment")
public class CapsulePod extends KubernetesDependentResource<Pod, ConsensusCapsule>
        implements SecondaryToPrimaryMapper<Pod> {
    public CapsulePod() {
        super(Pod.class);
    }

    @Override
    public Set<ResourceID> toPrimaryResourceIDs(Pod resource) {
        String name = resource.getMetadata().getLabels().get(LABEL_NODE_NAME);
        return null == name
                ? Collections.emptySet()
                : Set.of(new ResourceID(name, resource.getMetadata().getNamespace()));
    }
}
