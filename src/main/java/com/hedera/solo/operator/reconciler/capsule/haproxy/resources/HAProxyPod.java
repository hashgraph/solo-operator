// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.capsule.haproxy.resources;

import static com.hedera.solo.operator.library.model.Labels.hasLabel;

import com.hedera.solo.operator.model.v1alpha1.HAProxyCapsule;
import com.hedera.solo.operator.reconciler.common.Constants;
import com.hedera.solo.operator.reconciler.common.Metadata;
import io.fabric8.kubernetes.api.model.Pod;
import io.javaoperatorsdk.operator.api.config.informer.InformerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.Context;
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

/**
 * The HAProxyPod class is a KubernetesDependentResource that manages the lifecycle of HAProxy pods in a Kubernetes
 * cluster. It interacts with the HAProxyCapsule resource to ensure that the corresponding pods are correctly created,
 * updated, and managed.
 */
@KubernetesDependent
public class HAProxyPod extends KubernetesDependentResource<Pod, HAProxyCapsule>
        implements SecondaryToPrimaryMapper<Pod> {

    /**
     * Constructs a new HAProxyPod instance. Initializes the dependent resource to manage Pod objects and configures the
     * informer with a label selector for HAProxy pods.
     */
    public HAProxyPod() {
        super(Pod.class);
        configureWith(new KubernetesDependentResourceConfigBuilder<Pod>()
                .withKubernetesDependentInformerConfig(InformerConfiguration.builder(Pod.class)
                        .withLabelSelector((Constants.LABEL_SELECTOR_HAPROXY_APP))
                        .build())
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
    public Optional<Pod> getSecondaryResource(final HAProxyCapsule primary, final Context<HAProxyCapsule> context) {
        String namespace = primary.getMetadata().getNamespace();
        String nodeName = Metadata.prepareNodeName(primary.getSpec().getNodeId());
        return eventSource().flatMap(es -> es.list(namespace)
                .filter(hasLabel(Constants.LABEL_NODE_NAME, nodeName))
                .max(Comparator.comparing(
                        (Pod pod) -> ZonedDateTime.parse(pod.getMetadata().getCreationTimestamp())
                                .toEpochSecond())));
    }
}
