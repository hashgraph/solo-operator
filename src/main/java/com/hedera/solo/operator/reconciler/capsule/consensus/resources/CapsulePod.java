// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.capsule.consensus.resources;

import static com.hedera.solo.operator.reconciler.common.Constants.LABEL_NODE_NAME;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import io.fabric8.kubernetes.api.model.Pod;
import io.javaoperatorsdk.operator.api.config.informer.InformerConfiguration;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.event.ResourceID;
import io.javaoperatorsdk.operator.processing.event.source.PrimaryToSecondaryMapper;
import io.javaoperatorsdk.operator.processing.event.source.SecondaryToPrimaryMapper;
import java.time.ZonedDateTime;
import java.util.Collections;
import java.util.Comparator;
import java.util.Set;

@KubernetesDependent(
        labelSelector =
                "app.kubernetes.io/managed-by=solo-operator,app.kubernetes.io/part-of=consensus-node,app.kubernetes.io/component=deployment")
public class CapsulePod extends KubernetesDependentResource<Pod, ConsensusCapsule>
        implements PrimaryToSecondaryMapper<ConsensusCapsule>, SecondaryToPrimaryMapper<Pod> {
    public CapsulePod() {
        super(Pod.class);
    }

    @Override
    public Set<ResourceID> toPrimaryResourceIDs(Pod resource) {
        // convention: nodeName is the same as the ConsensusCapsule name
        String nodeName = resource.getMetadata().getLabels().get(LABEL_NODE_NAME);
        return null == nodeName
                ? Collections.emptySet()
                : Set.of(new ResourceID(nodeName, resource.getMetadata().getNamespace()));
    }

    @Override
    protected InformerConfiguration.InformerConfigurationBuilder<Pod> informerConfigurationBuilder() {
        return super.informerConfigurationBuilder().withPrimaryToSecondaryMapper(this);
    }

    @Override
    public Set<ResourceID> toSecondaryResourceIDs(final ConsensusCapsule primary) {
        // convention: nodeName is the same as the ConsensusCapsule name, so need to filter by nodeName
        String nodeName = primary.getMetadata().getName();
        return eventSource()
                .orElseThrow()
                .list(primary.getMetadata().getNamespace())
                .filter(pod -> nodeName.equals(pod.getMetadata().getLabels().get(LABEL_NODE_NAME)))
                .max(Comparator.comparing(
                        pod -> ZonedDateTime.parse(pod.getMetadata().getCreationTimestamp())))
                .map(ResourceID::fromResource)
                .map(Set::of)
                .orElse(Collections.emptySet());
    }
}
