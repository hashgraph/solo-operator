// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkinitialization.resources;

import com.hedera.solo.operator.model.v1alpha1.NetworkInitialization;
import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.reconciler.networkinitialization.NetworkInitializationReconciler;
import io.javaoperatorsdk.operator.api.config.informer.InformerConfiguration;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.event.ResourceID;
import io.javaoperatorsdk.operator.processing.event.source.PrimaryToSecondaryMapper;
import io.javaoperatorsdk.operator.processing.event.source.SecondaryToPrimaryMapper;
import java.util.Set;

@KubernetesDependent
public class NetworkInitializationOrbit extends KubernetesDependentResource<Orbit, NetworkInitialization>
        implements PrimaryToSecondaryMapper<NetworkInitialization>, SecondaryToPrimaryMapper<Orbit> {

    private final NetworkInitializationReconciler reconciler;

    public NetworkInitializationOrbit(final NetworkInitializationReconciler reconciler) {
        super(Orbit.class);
        this.reconciler = reconciler;
    }

    @Override
    public Set<ResourceID> toPrimaryResourceIDs(final Orbit resource) {
        return reconciler.toPrimaryResourceIDs(resource.getMetadata().getName());
    }

    @Override
    protected InformerConfiguration.InformerConfigurationBuilder<Orbit> informerConfigurationBuilder() {
        return super.informerConfigurationBuilder().withPrimaryToSecondaryMapper(this);
    }

    @Override
    public Set<ResourceID> toSecondaryResourceIDs(final NetworkInitialization primary) {
        return Set.of(new ResourceID(primary.getMetadata().getNamespace()));
    }
}
