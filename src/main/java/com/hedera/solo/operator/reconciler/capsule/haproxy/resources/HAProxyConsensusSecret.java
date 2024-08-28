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

import static com.hedera.solo.operator.library.model.Labels.hasLabel;
import static com.hedera.solo.operator.reconciler.capsule.consensus.ConsensusCapsuleConstants.LABEL_CONSENSUS_CAPSULE_NAME_KEY;
import static com.hedera.solo.operator.reconciler.capsule.consensus.ConsensusCapsuleConstants.LABEL_SELECTOR_CONSENSUS_NODE;
import static com.hedera.solo.operator.reconciler.capsule.consensus.ConsensusCapsuleReconciler.HAS_LABEL_SELECTOR_CONSENSUS_NODE;

import com.hedera.solo.operator.model.v1alpha1.HAProxyCapsule;
import com.hedera.solo.operator.reconciler.capsule.haproxy.HAProxyCapsuleReconciler;
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
                .filter(hasLabel(
                        LABEL_CONSENSUS_CAPSULE_NAME_KEY,
                        primary.getSpec().getConsensus().getNode()))
                .findAny()
                .map(ResourceID::fromResource)
                .map(Set::of)
                .orElse(Collections.emptySet());
    }
}
