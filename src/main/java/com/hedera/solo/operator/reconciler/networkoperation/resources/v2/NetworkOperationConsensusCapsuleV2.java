// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.resources.v2;

import com.hedera.solo.operator.library.josdk.resource.AbstractKubernetesBulkDependentResource;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.reconciler.networkoperation.NetworkOperationMediator;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResourceConfigBuilder;
import io.javaoperatorsdk.operator.processing.event.ResourceID;
import io.javaoperatorsdk.operator.processing.event.source.SecondaryToPrimaryMapper;
import java.util.HashMap;
import java.util.Map;
import java.util.Objects;
import java.util.Set;

@KubernetesDependent
public class NetworkOperationConsensusCapsuleV2
        extends AbstractKubernetesBulkDependentResource<ConsensusCapsule, NetworkOperation>
        implements SecondaryToPrimaryMapper<ConsensusCapsule> {
    private final NetworkOperationMediator mediator;

    public NetworkOperationConsensusCapsuleV2(final NetworkOperationMediator mediator) {
        super(ConsensusCapsule.class);
        this.mediator = mediator;
        configureWith(new KubernetesDependentResourceConfigBuilder<ConsensusCapsule>()
                .withUseSSA(Boolean.FALSE)
                .build());
    }

    @Override
    public Map<String, ConsensusCapsule> desiredResources(
            final NetworkOperation primary, final Context<NetworkOperation> context) {
        return getSecondaryResources(primary, context);
    }

    @Override
    public Set<ResourceID> toPrimaryResourceIDs(final ConsensusCapsule resource) {
        return mediator.primaryResourceIDs(resource.getMetadata().getNamespace());
    }

    @Override
    public Map<String, ConsensusCapsule> getSecondaryResources(
            final NetworkOperation primary, final Context<NetworkOperation> context) {
        return filterSecondaryResources(primary, new HashMap<>(super.getSecondaryResources(primary, context)));
    }

    public static Map<String, ConsensusCapsule> filterSecondaryResources(
            final NetworkOperation primary, final Map<String, ConsensusCapsule> secondaryResources) {

        // remove consensus nodes that are not in the network operation status
        if (Objects.nonNull(primary.getStatus())
                && Objects.nonNull(primary.getStatus().getConsensusCapsules())) {
            secondaryResources.values().removeIf(r -> !primary.getStatus()
                    .getConsensusCapsules()
                    .contains(r.getMetadata().getName()));
        }

        return secondaryResources;
    }
}
