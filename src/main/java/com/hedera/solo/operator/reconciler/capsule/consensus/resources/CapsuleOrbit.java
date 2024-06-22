package com.hedera.solo.operator.reconciler.capsule.consensus.resources;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.Orbit;
import io.javaoperatorsdk.operator.api.config.informer.InformerConfiguration;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.event.ResourceID;
import io.javaoperatorsdk.operator.processing.event.source.SecondaryToPrimaryMapper;
import java.util.Set;

@KubernetesDependent
public class CapsuleOrbit extends KubernetesDependentResource<Orbit, ConsensusCapsule>
        implements SecondaryToPrimaryMapper<Orbit>
{
    public CapsuleOrbit() {
        super(Orbit.class);
    }

    @Override
    protected InformerConfiguration.InformerConfigurationBuilder<Orbit> informerConfigurationBuilder() {
        return super.informerConfigurationBuilder().withPrimaryToSecondaryMapper(p -> Set.of(new ResourceID(p.getMetadata().getNamespace())));
    }

    @Override
    public Set<ResourceID> toPrimaryResourceIDs(Orbit resource) {
        return Set.of(
//                new ResourceID("node0", resource.getMetadata().getName()),
//                new ResourceID("node1", resource.getMetadata().getName()),
//                new ResourceID("node2", resource.getMetadata().getName())
        );
    }
}
