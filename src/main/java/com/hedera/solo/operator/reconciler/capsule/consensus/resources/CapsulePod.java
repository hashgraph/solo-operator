// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.capsule.consensus.resources;

import static com.hedera.solo.operator.reconciler.common.Constants.LABEL_NODE_NAME;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.reconciler.common.Metadata;
import io.fabric8.kubernetes.api.model.Pod;
import io.javaoperatorsdk.operator.api.config.informer.Informer;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.event.ResourceID;
import io.javaoperatorsdk.operator.processing.event.source.SecondaryToPrimaryMapper;
import java.time.ZonedDateTime;
import java.util.Collections;
import java.util.Comparator;
import java.util.Optional;
import java.util.Set;

/**
 * The CapsulePod class is a KubernetesDependentResource that maps secondary Pod resources to their corresponding
 * primary ConsensusCapsule resource.
 */
@KubernetesDependent(
        informer =
                @Informer(
                        labelSelector =
                                "app.kubernetes.io/managed-by=solo-operator,app.kubernetes.io/part-of=consensus-node,app.kubernetes.io/component=deployment"))
public class CapsulePod extends KubernetesDependentResource<Pod, ConsensusCapsule>
        implements SecondaryToPrimaryMapper<Pod> {
    /** Default constructor for the CapsulePod class. Initializes the resource type to Pod. */
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
    public Optional<Pod> getSecondaryResource(final ConsensusCapsule primary, final Context<ConsensusCapsule> context) {
        String nodeName = Metadata.prepareNodeName(primary.getSpec().getNodeId());
        return eventSource().flatMap(es -> es.list(primary.getMetadata().getNamespace())
                .filter(pod -> nodeName.equals(pod.getMetadata().getLabels().get(LABEL_NODE_NAME)))
                .max(Comparator.comparing(
                        pod -> ZonedDateTime.parse(pod.getMetadata().getCreationTimestamp()))));
    }
}
