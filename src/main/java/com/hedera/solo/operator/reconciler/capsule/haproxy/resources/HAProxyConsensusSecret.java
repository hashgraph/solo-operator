// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.capsule.haproxy.resources;

import static com.hedera.solo.operator.library.model.Labels.hasLabel;
import static com.hedera.solo.operator.library.model.Labels.hasLabels;
import static com.hedera.solo.operator.reconciler.capsule.consensus.ConsensusCapsuleConstants.LABEL_CONSENSUS_CAPSULE_KEY_TYPE;
import static com.hedera.solo.operator.reconciler.capsule.consensus.ConsensusCapsuleConstants.LABEL_CONSENSUS_CAPSULE_NAME_KEY;
import static com.hedera.solo.operator.reconciler.capsule.consensus.ConsensusCapsuleConstants.LABEL_SELECTOR_CONSENSUS_NODE;
import static com.hedera.solo.operator.reconciler.capsule.consensus.ConsensusCapsuleReconciler.HAS_LABEL_SELECTOR_CONSENSUS_NODE;

import com.hedera.solo.operator.model.v1alpha1.HAProxyCapsule;
import com.hedera.solo.operator.reconciler.capsule.haproxy.HAProxyCapsuleReconciler;
import com.hedera.solo.operator.reconciler.common.Constants;
import io.fabric8.kubernetes.api.model.Secret;
import io.javaoperatorsdk.operator.api.config.informer.InformerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ResourceDiscriminator;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResourceConfigBuilder;
import io.javaoperatorsdk.operator.processing.event.ResourceID;
import io.javaoperatorsdk.operator.processing.event.source.PrimaryToSecondaryMapper;
import io.javaoperatorsdk.operator.processing.event.source.SecondaryToPrimaryMapper;
import java.util.Collections;
import java.util.Map;
import java.util.Optional;
import java.util.Set;
import java.util.stream.Stream;

@KubernetesDependent
public class HAProxyConsensusSecret extends KubernetesDependentResource<Secret, HAProxyCapsule>
        implements PrimaryToSecondaryMapper<HAProxyCapsule>,
                ResourceDiscriminator<Secret, HAProxyCapsule>,
                SecondaryToPrimaryMapper<Secret> {
    private final HAProxyCapsuleReconciler reconciler;

    public HAProxyConsensusSecret(final HAProxyCapsuleReconciler reconciler) {
        super(Secret.class);
        this.reconciler = reconciler;
        configureWith(new KubernetesDependentResourceConfigBuilder<Secret>()
                .withLabelSelector(LABEL_SELECTOR_CONSENSUS_NODE)
                .withResourceDiscriminator(this)
                .build());
    }

    @Override
    public Optional<Secret> distinguish(
            final Class<Secret> resource, final HAProxyCapsule primary, final Context<HAProxyCapsule> context) {
        return context.getSecondaryResources(Secret.class).stream()
                .filter(HAS_LABEL_SELECTOR_CONSENSUS_NODE)
                .findAny();
    }

    @Override
    protected InformerConfiguration.InformerConfigurationBuilder<Secret> informerConfigurationBuilder() {
        return super.informerConfigurationBuilder().withPrimaryToSecondaryMapper(this);
    }

    @Override
    public Set<ResourceID> toPrimaryResourceIDs(final Secret resource) {
        return reconciler
                .eventSourceContext()
                .getPrimaryCache()
                .list(resource.getMetadata().getNamespace())
                .filter(p -> hasLabel(
                        resource,
                        LABEL_CONSENSUS_CAPSULE_NAME_KEY,
                        p.getSpec().getConsensus().getNode()))
                .filter(p -> hasLabel(resource, LABEL_CONSENSUS_CAPSULE_KEY_TYPE, Constants.KEY_TYPE_GRPC_TLS))
                .findAny()
                .map(ResourceID::fromResource)
                .map(Set::of)
                .orElse(Collections.emptySet());
    }

    @Override
    public Set<ResourceID> toSecondaryResourceIDs(final HAProxyCapsule primary) {
        return eventSource()
                .map(es -> es.list(primary.getMetadata().getNamespace()))
                .orElse(Stream.empty())
                .filter(hasLabels(Map.of(
                        LABEL_CONSENSUS_CAPSULE_NAME_KEY,
                        primary.getSpec().getConsensus().getNode(),
                        LABEL_CONSENSUS_CAPSULE_KEY_TYPE,
                        Constants.KEY_TYPE_GRPC_TLS)))
                .findAny()
                .map(ResourceID::fromResource)
                .map(Set::of)
                .orElse(Collections.emptySet());
    }
}
