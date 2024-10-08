package com.hedera.solo.operator.reconciler.networkinitialization.resources;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;

import com.hedera.solo.operator.model.v1alpha1.NetworkInitialization;
import com.hedera.solo.operator.reconciler.networkinitialization.NetworkInitializationReconciler;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.javaoperatorsdk.operator.api.config.informer.InformerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ResourceDiscriminator;
import io.javaoperatorsdk.operator.processing.dependent.Creator;
import io.javaoperatorsdk.operator.processing.dependent.Updater;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResourceConfigBuilder;
import io.javaoperatorsdk.operator.processing.event.ResourceID;
import io.javaoperatorsdk.operator.processing.event.source.PrimaryToSecondaryMapper;
import io.javaoperatorsdk.operator.processing.event.source.SecondaryToPrimaryMapper;
import java.lang.invoke.MethodHandles;
import java.util.Collections;
import java.util.Optional;
import java.util.Set;
import java.util.stream.Collectors;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

@KubernetesDependent
public class NetworkInitializationState extends KubernetesDependentResource<ConfigMap, NetworkInitialization>
        implements
//        Creator<ConfigMap, NetworkInitialization>,
//        Updater<ConfigMap, NetworkInitialization>,
//        PrimaryToSecondaryMapper<NetworkInitialization>,
        SecondaryToPrimaryMapper<ConfigMap>,
        ResourceDiscriminator<ConfigMap, NetworkInitialization>
{
    Logger log = LoggerFactory.getLogger(MethodHandles.lookup().lookupClass());
    public static final String NAME = "network-initialization-state";
    private final NetworkInitializationReconciler reconciler;

    public NetworkInitializationState(NetworkInitializationReconciler reconciler) {
        super(ConfigMap.class);
        this.reconciler = reconciler;
        configureWith(new KubernetesDependentResourceConfigBuilder<ConfigMap>()
                .withLabelSelector("app=consensus,type=state")
                .withResourceDiscriminator(this)
                .build());
    }

//    @Override
//    protected InformerConfiguration.InformerConfigurationBuilder<ConfigMap> informerConfigurationBuilder() {
//        return super.informerConfigurationBuilder().withPrimaryToSecondaryMapper(this);
//    }

//    @Override
//    protected ConfigMap desired(NetworkInitialization primary, Context<NetworkInitialization> context) {
//        log.info("NetworkInitializationState::desired {} {}", primary.getMetadata().getName(), context.getSecondaryResource(ConfigMap.class, this).isPresent());
//        return edit(context.getSecondaryResource(ConfigMap.class, this).orElseGet(ConfigMap::new))
//                .with(builder -> builder
//                        .editMetadata()
//                        .withNamespace(primary.getMetadata().getNamespace())
//                        .withName(NAME)
//                        .addToLabels("app", "consensus")
//                        .addToLabels("type", "state"))
//                .build();
//    }

//    @Override
//    public Set<ResourceID> toSecondaryResourceIDs(NetworkInitialization primary) {
//        return eventSource().orElseThrow().list(primary.getMetadata().getNamespace()).map(ResourceID::fromResource).collect(Collectors.toSet());
//    }

    @Override
    public Optional<ConfigMap> distinguish(Class<ConfigMap> resource, NetworkInitialization primary, Context<NetworkInitialization> context) {
        return eventSource().orElseThrow().list(primary.getMetadata().getNamespace()).findAny();
    }

    @Override
    public Set<ResourceID> toPrimaryResourceIDs(ConfigMap resource) {
        Set<ResourceID> set = reconciler.eventSourceContext().getPrimaryCache().list(resource.getMetadata().getNamespace())
                .filter(reconciler.active())
                .min(reconciler.prioritize())
                .map(ResourceID::fromResource)
                .map(Set::of)
                .orElseGet(Collections::emptySet);
        log.info("NetworkInitializationState::toPrimaryResourceIDs {}", set);
        return set;
    }
}
