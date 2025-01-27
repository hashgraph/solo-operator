// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.networkoperation.resources.v2;

import com.hedera.solo.operator.model.v1alpha1.NetworkOperation;
import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.reconciler.networkoperation.NetworkOperationMediator;
import io.javaoperatorsdk.operator.api.config.informer.InformerConfiguration;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.event.ResourceID;
import io.javaoperatorsdk.operator.processing.event.source.PrimaryToSecondaryMapper;
import io.javaoperatorsdk.operator.processing.event.source.SecondaryToPrimaryMapper;
import java.util.Set;

@KubernetesDependent
public class NetworkOperationOrbitV2 extends KubernetesDependentResource<Orbit, NetworkOperation>
        implements PrimaryToSecondaryMapper<NetworkOperation>, SecondaryToPrimaryMapper<Orbit> {

    private final NetworkOperationMediator mediator;

    public NetworkOperationOrbitV2(final NetworkOperationMediator mediator) {
        super(Orbit.class);
        this.mediator = mediator;
    }

    @Override
    public Set<ResourceID> toPrimaryResourceIDs(final Orbit resource) {
        return mediator.primaryResourceIDs(resource.getMetadata().getName());
    }

    @Override
    protected InformerConfiguration.InformerConfigurationBuilder<Orbit> informerConfigurationBuilder() {
        return super.informerConfigurationBuilder().withPrimaryToSecondaryMapper(this);
    }

    @Override
    public Set<ResourceID> toSecondaryResourceIDs(final NetworkOperation primary) {
        return Set.of(new ResourceID(primary.getMetadata().getNamespace()));
    }
}
