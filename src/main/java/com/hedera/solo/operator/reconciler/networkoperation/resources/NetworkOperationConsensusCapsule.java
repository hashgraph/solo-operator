// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.resources;

import com.hedera.solo.operator.library.josdk.resource.AbstractKubernetesBulkDependentResource;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.reconciler.networkoperation.NetworkOperationReconciler;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResourceConfigBuilder;
import io.javaoperatorsdk.operator.processing.event.ResourceID;
import io.javaoperatorsdk.operator.processing.event.source.SecondaryToPrimaryMapper;
import java.util.HashMap;
import java.util.Map;
import java.util.Set;

@KubernetesDependent
public class NetworkOperationConsensusCapsule
        extends AbstractKubernetesBulkDependentResource<ConsensusCapsule, NetworkOperation>
        implements SecondaryToPrimaryMapper<ConsensusCapsule> {
    private final NetworkOperationReconciler reconciler;

    public NetworkOperationConsensusCapsule(final NetworkOperationReconciler reconciler) {
        super(ConsensusCapsule.class);
        this.reconciler = reconciler;
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
        return reconciler.toPrimaryResourceIDs(resource.getMetadata().getNamespace());
    }

    @Override
    public Map<String, ConsensusCapsule> getSecondaryResources(
            final NetworkOperation primary, final Context<NetworkOperation> context) {
        return getSecondaryResources(primary, new HashMap<>(super.getSecondaryResources(primary, context)));
    }

    public static Map<String, ConsensusCapsule> getSecondaryResources(
            final NetworkOperation primary, final Map<String, ConsensusCapsule> secondaryResources) {
        if (null != primary.getStatus() && null != primary.getStatus().getConsensusCapsules()) {
            secondaryResources.values().removeIf(r -> !primary.getStatus()
                    .getConsensusCapsules()
                    .contains(r.getMetadata().getName()));
        }
        return secondaryResources;
    }
}
