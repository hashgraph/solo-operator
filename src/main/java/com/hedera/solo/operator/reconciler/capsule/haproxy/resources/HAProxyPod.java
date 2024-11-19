// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.capsule.haproxy.resources;

import com.hedera.solo.operator.model.v1alpha1.HAProxyCapsule;
import com.hedera.solo.operator.reconciler.common.Constants;
import io.fabric8.kubernetes.api.model.Pod;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ResourceDiscriminator;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResourceConfigBuilder;
import io.javaoperatorsdk.operator.processing.event.ResourceID;
import io.javaoperatorsdk.operator.processing.event.source.SecondaryToPrimaryMapper;
import java.time.ZonedDateTime;
import java.util.Collections;
import java.util.Comparator;
import java.util.Optional;
import java.util.Set;

@KubernetesDependent
public class HAProxyPod extends KubernetesDependentResource<Pod, HAProxyCapsule>
        implements SecondaryToPrimaryMapper<Pod>, ResourceDiscriminator<Pod, HAProxyCapsule> {
    public HAProxyPod() {
        super(Pod.class);
        configureWith(new KubernetesDependentResourceConfigBuilder<Pod>()
                .withLabelSelector(Constants.LABEL_SELECTOR_APP_HAPROXY)
                .withResourceDiscriminator(this)
                .build());
    }

    @Override
    public Set<ResourceID> toPrimaryResourceIDs(Pod resource) {
        String name = resource.getMetadata().getLabels().get(Constants.COMPONENT_HAPROXY);
        return null == name
                ? Collections.emptySet()
                : Set.of(new ResourceID(name, resource.getMetadata().getNamespace()));
    }

    @Override
    public Optional<Pod> distinguish(
            final Class<Pod> resource, final HAProxyCapsule primary, final Context<HAProxyCapsule> context) {
        return context.getSecondaryResourcesAsStream(resource)
                .max(Comparator.comparing(
                        (Pod pod) -> ZonedDateTime.parse(pod.getMetadata().getCreationTimestamp())
                                .toEpochSecond()));
    }
}
