// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.capsule.haproxy.resources;

import static com.hedera.solo.operator.library.model.Labels.hasLabel;
import static com.hedera.solo.operator.library.model.Labels.hasLabels;
import static com.hedera.solo.operator.reconciler.common.Constants.LABEL_SELECTOR_CONSENSUS_NODE_APP;

import com.hedera.solo.operator.model.v1alpha1.HAProxyCapsule;
import com.hedera.solo.operator.reconciler.capsule.haproxy.HAProxyCapsuleReconciler;
import com.hedera.solo.operator.reconciler.common.Constants;
import com.hedera.solo.operator.reconciler.common.Metadata;
import io.fabric8.kubernetes.api.model.Service;
import io.javaoperatorsdk.operator.api.config.informer.InformerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResourceConfigBuilder;
import io.javaoperatorsdk.operator.processing.event.ResourceID;
import io.javaoperatorsdk.operator.processing.event.source.SecondaryToPrimaryMapper;
import java.util.Collections;
import java.util.Map;
import java.util.Optional;
import java.util.Set;

/**
 * The HAProxyConsensusService class is responsible for mapping secondary resources (Services) to primary resources
 * (HAProxyCapsule).
 */
@KubernetesDependent
public class HAProxyConsensusService extends KubernetesDependentResource<Service, HAProxyCapsule>
        implements SecondaryToPrimaryMapper<Service> {
    private final HAProxyCapsuleReconciler reconciler;

    /**
     * Constructs a new HAProxyConsensusService with the given HAProxyCapsuleReconciler.
     *
     * @param reconciler The HAProxyCapsuleReconciler instance for handling HAProxyCapsule resources.
     */
    public HAProxyConsensusService(final HAProxyCapsuleReconciler reconciler) {
        super(Service.class);
        this.reconciler = reconciler;
        configureWith(new KubernetesDependentResourceConfigBuilder<Service>()
                .withKubernetesDependentInformerConfig(InformerConfiguration.builder(Service.class)
                        .withLabelSelector((LABEL_SELECTOR_CONSENSUS_NODE_APP))
                        .build())
                .build());
    }

    @Override
    public Set<ResourceID> toPrimaryResourceIDs(final Service resource) {
        return reconciler
                .eventSourceContext()
                .getPrimaryCache()
                .list(resource.getMetadata().getNamespace())
                .filter(p -> hasLabel(
                        resource,
                        Constants.LABEL_NODE_NAME,
                        Metadata.prepareNodeName(p.getSpec().getNodeId())))
                .findAny()
                .map(ResourceID::fromResource)
                .map(Set::of)
                .orElse(Collections.emptySet());
    }

    @Override
    public Optional<Service> getSecondaryResource(final HAProxyCapsule primary, final Context<HAProxyCapsule> context) {
        final String nodeName = Metadata.prepareNodeName(primary.getSpec().getNodeId());
        final String namespace = primary.getMetadata().getNamespace();
        return eventSource().flatMap(es -> es.list(namespace)
                .filter(hasLabels(Map.of(Constants.LABEL_NODE_NAME, nodeName)))
                .findAny());
    }
}
