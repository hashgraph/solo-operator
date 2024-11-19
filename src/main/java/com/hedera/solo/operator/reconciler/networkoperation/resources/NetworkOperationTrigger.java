// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.resources;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;

import com.hedera.solo.operator.library.numeric.RolloverComparator;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.reconciler.common.Constants;
import com.hedera.solo.operator.reconciler.common.Metadata;
import com.hedera.solo.operator.reconciler.networkoperation.NetworkOperationReconciler;
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
public class NetworkOperationTrigger extends KubernetesDependentResource<ConfigMap, NetworkOperation>
        implements ResourceDiscriminator<ConfigMap, NetworkOperation>, SecondaryToPrimaryMapper<ConfigMap> {
    private final AtomicLong sequenceNumber = new AtomicLong();
    private final RolloverComparator rolloverComparator = new RolloverComparator(63);
    private static final String NAME = "network-operation-trigger";
    private final NetworkOperationReconciler reconciler;

    public NetworkOperationTrigger(final NetworkOperationReconciler reconciler) {
        super(ConfigMap.class);
        this.reconciler = reconciler;
        configureWith(new KubernetesDependentResourceConfigBuilder<ConfigMap>()
                .withLabelSelector(Constants.LABEL_SELECTOR_NET_OP_TRIGGER)
                .withResourceDiscriminator(this)
                .build());
    }

    public synchronized void next(final NetworkOperation resource, final Context<NetworkOperation> context) {
        try {
            final String orbitName = resource.getSpec().getOrbit();
            context.getClient()
                    .resource(edit(new ConfigMap())
                            .with(builder -> Metadata.applyStandardLabels(
                                            builder.editMetadata(),
                                            NAME,
                                            orbitName,
                                            Constants.COMPONENT_NET_OP,
                                            Constants.COMPONENT_TRIGGER,
                                            null)
                                    .withName(NAME)
                                    .withNamespace(resource.getMetadata().getNamespace()))
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
            final Class<ConfigMap> resource, final NetworkOperation primary, final Context<NetworkOperation> context) {
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
