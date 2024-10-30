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

package com.hedera.solo.operator.reconciler.networkinitialization.resources;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;

import com.hedera.solo.operator.library.numeric.RolloverComparator;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitialization;
import com.hedera.solo.operator.reconciler.networkinitialization.NetworkInitializationReconciler;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.client.KubernetesClientException;
import io.fabric8.kubernetes.client.dsl.Updatable;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ResourceDiscriminator;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResourceConfigBuilder;
import io.javaoperatorsdk.operator.processing.event.ResourceID;
import io.javaoperatorsdk.operator.processing.event.source.SecondaryToPrimaryMapper;
import java.util.Optional;
import java.util.Set;
import java.util.concurrent.atomic.AtomicLong;

@KubernetesDependent
public class NetworkInitializationTrigger extends KubernetesDependentResource<ConfigMap, NetworkInitialization>
        implements ResourceDiscriminator<ConfigMap, NetworkInitialization>, SecondaryToPrimaryMapper<ConfigMap> {
    private final AtomicLong sequenceNumber = new AtomicLong();
    private final RolloverComparator rolloverComparator = new RolloverComparator(63);
    private static final String NAME = "network-initialization-trigger";
    private final NetworkInitializationReconciler reconciler;

    public NetworkInitializationTrigger(final NetworkInitializationReconciler reconciler) {
        super(ConfigMap.class);
        this.reconciler = reconciler;
        configureWith(new KubernetesDependentResourceConfigBuilder<ConfigMap>()
                .withLabelSelector("app=consensus,type=trigger")
                .withResourceDiscriminator(this)
                .build());
    }

    public synchronized void next(final NetworkInitialization resource, final Context<NetworkInitialization> context) {
        try {
            context.getClient()
                    .resource(edit(new ConfigMap())
                            .with(metadata -> metadata.editMetadata()
                                    .withName(NAME)
                                    .withNamespace(resource.getMetadata().getNamespace())
                                    .addToLabels("app", "consensus")
                                    .addToLabels("type", "trigger"))
                            .with(data -> data.addToData(
                                    "count",
                                    String.valueOf(rolloverComparator.rollover(sequenceNumber.getAndIncrement()))))
                            .build())
                    .createOr(Updatable::update);
        } catch (KubernetesClientException ignored) {
        }
    }

    @Override
    public Optional<ConfigMap> distinguish(
            final Class<ConfigMap> resource,
            final NetworkInitialization primary,
            final Context<NetworkInitialization> context) {
        return eventSource()
                .orElseThrow()
                .list(primary.getMetadata().getNamespace())
                .findAny();
    }

    @Override
    public Set<ResourceID> toPrimaryResourceIDs(final ConfigMap resource) {
        return reconciler.toPrimaryResourceIDs(resource.getMetadata().getNamespace());
    }
}
