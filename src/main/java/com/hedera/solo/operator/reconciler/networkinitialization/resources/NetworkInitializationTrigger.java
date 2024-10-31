// SPDX-License-Identifier: Apache-2.0

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
