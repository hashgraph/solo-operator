package com.hedera.solo.test.ni2;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;

import com.hedera.solo.operator.model.v1alpha1.NetworkInitialization;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitializationSequence;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitializationStatus;
import io.fabric8.kubernetes.api.model.HasMetadata;
import io.fabric8.kubernetes.api.model.Secret;
import io.javaoperatorsdk.operator.api.config.informer.InformerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.Updater;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.BooleanWithUndefined;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResourceConfigBuilder;
import io.javaoperatorsdk.operator.processing.event.ResourceID;
import io.javaoperatorsdk.operator.processing.event.source.PrimaryToSecondaryMapper;
import io.javaoperatorsdk.operator.processing.event.source.SecondaryToPrimaryMapper;
import java.util.Collections;
import java.util.Comparator;
import java.util.Set;

@KubernetesDependent//(useSSA = BooleanWithUndefined.FALSE)
public class NIDR extends KubernetesDependentResource<NetworkInitialization, NetworkInitializationSequence>
implements
        Updater<NetworkInitialization, NetworkInitializationSequence>,
        PrimaryToSecondaryMapper<NetworkInitializationSequence>,
        SecondaryToPrimaryMapper<NetworkInitialization>
{
    public NIDR() {
        super(NetworkInitialization.class);
        configureWith(new KubernetesDependentResourceConfigBuilder<NetworkInitialization>()
                .withUseSSA(false)
                .build());
    }

    @Override
    protected NetworkInitialization desired(NetworkInitializationSequence primary, Context<NetworkInitializationSequence> context) {
        NetworkInitialization ni =  edit(context.getSecondaryResource(NetworkInitialization.class).orElseThrow())
//                .with(builder -> builder
//                        .editMetadata()
//                        .withManagedFields())
//                .with(builder -> builder
//                        .editMetadata()
//                        .addToLabels("test-add-label", "ok"))
                .with(builder -> builder
                        .editStatus()
                        .withPhase(NetworkInitializationStatus.Phase.Succeeded))
                .build();
        return context.getClient().resource(ni).patchStatus();
    }

    @Override
    protected InformerConfiguration.InformerConfigurationBuilder<NetworkInitialization> informerConfigurationBuilder() {
        return super.informerConfigurationBuilder().withPrimaryToSecondaryMapper(this);
    }

    @Override
    protected NetworkInitialization handleUpdate(NetworkInitialization actual, NetworkInitialization desired, NetworkInitializationSequence primary, Context<NetworkInitializationSequence> context) {
        return super.handleUpdate(actual, desired, primary, context);
    }

    @Override
    public NetworkInitialization update(NetworkInitialization actual, NetworkInitialization desired, NetworkInitializationSequence primary, Context<NetworkInitializationSequence> context) {
        return super.update(actual, desired, primary, context);
    }

    @Override
    public Set<ResourceID> toSecondaryResourceIDs(NetworkInitializationSequence primary) {
        Set<NetworkInitializationStatus.Phase> phases = Set.of(NetworkInitializationStatus.Phase.Pending, NetworkInitializationStatus.Phase.Running);
        return eventSource().orElseThrow().list(primary.getMetadata().getNamespace())
                .filter(x -> null != x.getStatus())
                .filter(x -> phases.contains(x.getStatus().getPhase()))
                .min(Comparator.comparing(x -> x.getStatus().getSequenceNumber()))
                .map(ResourceID::fromResource)
                .map(Set::of)
                .orElse(Collections.emptySet());
    }

    @Override
    public Set<ResourceID> toPrimaryResourceIDs(NetworkInitialization resource) {
        return Set.of(new ResourceID(resource.getMetadata().getNamespace(), resource.getMetadata().getNamespace()));
    }

    // mappers should map to first in seq
}
