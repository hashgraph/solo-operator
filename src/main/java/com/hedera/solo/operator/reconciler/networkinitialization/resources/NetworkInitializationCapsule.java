package com.hedera.solo.operator.reconciler.networkinitialization.resources;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitialization;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.BulkDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.Updater;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.BooleanWithUndefined;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.event.ResourceID;
import io.javaoperatorsdk.operator.processing.event.source.SecondaryToPrimaryMapper;
import java.util.Map;
import java.util.Set;
import java.util.function.Function;
import java.util.stream.Collectors;

// why do I need SSA=false here, but not in ConsensusCapsuleReconciler?
@KubernetesDependent(useSSA = BooleanWithUndefined.FALSE)
public class NetworkInitializationCapsule
        extends
        KubernetesDependentResource<ConsensusCapsule, NetworkInitialization>
        implements
        BulkDependentResource<ConsensusCapsule, NetworkInitialization>,
        SecondaryToPrimaryMapper<ConsensusCapsule>,
        Updater<ConsensusCapsule, NetworkInitialization>
{
    public NetworkInitializationCapsule() {
        super(ConsensusCapsule.class);
    }

    @Override
    public Map<String, ConsensusCapsule> desiredResources(NetworkInitialization primary, Context<NetworkInitialization> context) {
        return getSecondaryResources(primary, context).entrySet().stream().collect(Collectors.toMap(Map.Entry::getKey,
                v -> v.getValue().edit().editSpec().withEligibleForDeployment(true).endSpec().build()
                ));
    }

    @Override
    public Map<String, ConsensusCapsule> getSecondaryResources(NetworkInitialization primary, Context<NetworkInitialization> context) {
        return context.getSecondaryResourcesAsStream(ConsensusCapsule.class).collect(Collectors.toMap($ -> $.getMetadata().getName(), Function.identity()));
    }

    @Override
    public Set<ResourceID> toPrimaryResourceIDs(ConsensusCapsule resource) {
        return Set.of(new ResourceID("network-initialization", resource.getMetadata().getNamespace()));
    }
}
