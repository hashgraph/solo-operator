package com.hedera.solo.operator.reconciler.networkinitialization.resources;

import com.hedera.solo.operator.model.v1alpha1.NetworkInitialization;
import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.reconciler.networkinitialization.NetworkInitializationReconciler;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.Secret;
import io.javaoperatorsdk.operator.api.config.informer.InformerConfiguration;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.event.ResourceID;
import io.javaoperatorsdk.operator.processing.event.source.PrimaryToSecondaryMapper;
import io.javaoperatorsdk.operator.processing.event.source.SecondaryToPrimaryMapper;
import java.lang.invoke.MethodHandles;
import java.util.Collections;
import java.util.Set;
import java.util.stream.Collectors;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

@KubernetesDependent
public class NetworkInitializationOrbit extends KubernetesDependentResource<Orbit, NetworkInitialization>
implements PrimaryToSecondaryMapper<NetworkInitialization>, SecondaryToPrimaryMapper<Orbit> {
    private static final Logger log = LoggerFactory.getLogger(MethodHandles.lookup().lookupClass());

    private final NetworkInitializationReconciler reconciler;
    public NetworkInitializationOrbit(NetworkInitializationReconciler reconciler) {
        super(Orbit.class);
        this.reconciler = reconciler;
    }

    @Override
    public Set<ResourceID> toPrimaryResourceIDs(Orbit resource) {
        Set<ResourceID> set = reconciler.eventSourceContext().getPrimaryCache().list(resource.getMetadata().getName())
                .filter(reconciler.active())
                .min(reconciler.prioritize())
                .map(ResourceID::fromResource)
                .map(Set::of)
                .orElseGet(Collections::emptySet);
        log.info("NetworkInitializationOrbit::toPrimaryResourceIDs {} -> {}", resource.getMetadata().getName(), set);
        return set;
    }

    @Override
    protected InformerConfiguration.InformerConfigurationBuilder<Orbit> informerConfigurationBuilder() {
        return super.informerConfigurationBuilder().withPrimaryToSecondaryMapper(this);
    }

    @Override
    public Set<ResourceID> toSecondaryResourceIDs(NetworkInitialization primary) {
        return Set.of(new ResourceID(primary.getMetadata().getNamespace()));
    }
}
