package com.hedera.solo.operator.reconciler.networkinitialization.resources;

import com.hedera.solo.operator.library.dependentresources.ReadOnlyKubernetesDependentResource;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitialization;
import com.hedera.solo.operator.reconciler.capsule.consensus.resources.CapsuleRegistry;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ResourceDiscriminator;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.event.ResourceID;
import java.util.Optional;

@KubernetesDependent
public class NetworkInitializationCapsules extends ReadOnlyKubernetesDependentResource<ConfigMap, NetworkInitialization>
        implements ResourceDiscriminator<ConfigMap, NetworkInitialization>
{
    public NetworkInitializationCapsules() {
        super(ConfigMap.class);
        setResourceDiscriminator(this);
    }

    @Override
    public Optional<ConfigMap> distinguish(Class<ConfigMap> aClass, NetworkInitialization primary, Context<NetworkInitialization> context) {
        ResourceID id = new ResourceID(CapsuleRegistry.NAME, primary.getMetadata().getNamespace());
        return context.getSecondaryResources(aClass).stream().filter(r -> id.equals(ResourceID.fromResource(r))).findAny();
    }
}
