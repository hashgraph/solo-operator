// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkinitialization.resources;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;

import com.hedera.solo.operator.library.josdk.resource.AbstractKubernetesBulkDependentResource;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitialization;
import com.hedera.solo.operator.reconciler.networkinitialization.NetworkInitializationReconciler;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.Updater;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResourceConfigBuilder;
import io.javaoperatorsdk.operator.processing.event.ResourceID;
import io.javaoperatorsdk.operator.processing.event.source.SecondaryToPrimaryMapper;
import java.util.HashMap;
import java.util.Map;
import java.util.Set;

@KubernetesDependent
public class NetworkInitializationConsensusCapsule
        extends AbstractKubernetesBulkDependentResource<ConsensusCapsule, NetworkInitialization>
        implements SecondaryToPrimaryMapper<ConsensusCapsule>, Updater<ConsensusCapsule, NetworkInitialization> {
    private final NetworkInitializationReconciler reconciler;

    public NetworkInitializationConsensusCapsule(final NetworkInitializationReconciler reconciler) {
        super(ConsensusCapsule.class);
        this.reconciler = reconciler;
        configureWith(new KubernetesDependentResourceConfigBuilder<ConsensusCapsule>()
                .withUseSSA(Boolean.FALSE)
                .build());
    }

    @Override
    public Map<String, ConsensusCapsule> desiredResources(
            final NetworkInitialization primary, final Context<NetworkInitialization> context) {
        Map<String, ConsensusCapsule> map = getSecondaryResources(primary, context);
        map.replaceAll((k, v) -> edit(v).with(spec -> spec.editSpec().withEligibleForDeployment())
                .build());
        return map;
    }

    @Override
    public Set<ResourceID> toPrimaryResourceIDs(final ConsensusCapsule resource) {
        return reconciler.toPrimaryResourceIDs(resource.getMetadata().getNamespace());
    }

    @Override
    public Map<String, ConsensusCapsule> getSecondaryResources(
            final NetworkInitialization primary, final Context<NetworkInitialization> context) {
        return getSecondaryResources(primary, new HashMap<>(super.getSecondaryResources(primary, context)));
    }

    public static Map<String, ConsensusCapsule> getSecondaryResources(
            final NetworkInitialization primary, final Map<String, ConsensusCapsule> secondaryResources) {
        if (null != primary.getStatus() && null != primary.getStatus().getConsensusCapsules()) {
            secondaryResources.values().removeIf(r -> !primary.getStatus()
                    .getConsensusCapsules()
                    .contains(r.getMetadata().getName()));
        }
        return secondaryResources;
    }
}
