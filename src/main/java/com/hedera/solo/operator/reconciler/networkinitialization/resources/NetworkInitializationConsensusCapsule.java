package com.hedera.solo.operator.reconciler.networkinitialization.resources;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitialization;
import io.javaoperatorsdk.operator.api.config.informer.InformerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ResourceDiscriminator;
import io.javaoperatorsdk.operator.processing.dependent.BulkDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.event.ResourceID;
import io.javaoperatorsdk.operator.processing.event.source.SecondaryToPrimaryMapper;
import java.util.Map;
import java.util.Optional;
import java.util.Set;
import java.util.function.Function;
import java.util.stream.Collectors;

@KubernetesDependent
public class NetworkInitializationConsensusCapsule extends KubernetesDependentResource<ConsensusCapsule, NetworkInitialization>
    implements
//        ResourceDiscriminator<ConsensusCapsule, NetworkInitialization>,
//        SecondaryToPrimaryMapper<ConsensusCapsule>,
        BulkDependentResource<ConsensusCapsule, NetworkInitialization>
{
    public NetworkInitializationConsensusCapsule() {
        super(ConsensusCapsule.class);
    }

    @Override
    public Map<String, ConsensusCapsule> desiredResources(NetworkInitialization primary, Context<NetworkInitialization> context) {
        return getSecondaryResources(primary, context);
    }

    @Override
    public Map<String, ConsensusCapsule> getSecondaryResources(NetworkInitialization primary, Context<NetworkInitialization> context) {
        return context.getClient().resources(ConsensusCapsule.class).inNamespace(primary.getMetadata().getNamespace()).list().getItems().stream().collect(Collectors.toMap(x -> x.getMetadata().getName(), Function.identity()));
    }

    @Override
    public void deleteTargetResource(NetworkInitialization primary, ConsensusCapsule resource, String key, Context<NetworkInitialization> context) {
        //super.deleteTargetResource(primary, resource, key, context);
    }
//    @Override
//    protected InformerConfiguration.InformerConfigurationBuilder<ConsensusCapsule> informerConfigurationBuilder() {
//        return super.informerConfigurationBuilder()
//                .withPrimaryToSecondaryMapper(p -> ((NetworkInitialization)p).getSpec().getConsensusCapsules().stream().map(name -> new ResourceID(name, p.getMetadata().getNamespace())).collect(Collectors.toSet()));
//    }

//    @Override
//    public Set<ResourceID> toPrimaryResourceIDs(ConsensusCapsule consensusCapsule) {
//        return Set.of();
//    }
//
//    @Override
//    public Optional<ConsensusCapsule> distinguish(Class<ConsensusCapsule> resource, NetworkInitialization primary, Context<NetworkInitialization> context) {
//        return context.getSecondaryResources();
//    }
}
