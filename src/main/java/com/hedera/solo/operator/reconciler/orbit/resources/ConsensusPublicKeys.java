package com.hedera.solo.operator.reconciler.orbit.resources;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;
import static com.hedera.solo.operator.reconciler.capsule.consensus.ConsensusCapsuleConstants.LABEL_SELECTOR_CONSENSUS_NODE;
import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;

import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.reconciler.common.Metadata;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.EventSource;
import io.fabric8.kubernetes.api.model.Service;
import io.javaoperatorsdk.operator.api.config.informer.InformerConfiguration;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ResourceDiscriminator;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDNoGCKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResourceConfigBuilder;
import java.lang.invoke.MethodHandles;
import java.util.Optional;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

@KubernetesDependent
public class ConsensusPublicKeys extends CRUDNoGCKubernetesDependentResource<ConfigMap, Orbit>
        implements
        ResourceDiscriminator<ConfigMap, Orbit>
{
    private static final Logger log = LoggerFactory.getLogger(MethodHandles.lookup().lookupClass());

    public ConsensusPublicKeys() {
        super(ConfigMap.class);
        configureWith(new KubernetesDependentResourceConfigBuilder<ConfigMap>()
                .withLabelSelector("public-keys")
                .withResourceDiscriminator(this)
                .build());

    }

    @Override
    protected ConfigMap desired(Orbit primary, Context<Orbit> context) {
        ConfigMap cm = context.getSecondaryResource(ConfigMap.class, this).orElse(new ConfigMap());

        return edit(cm)
                .with(builder -> Metadata.applyOrbitMetaTo(primary, builder.editMetadata()))
                .with(metadata -> metadata.editMetadata()
                        .withName("public-keys")
                        .withNamespace(primary.getMetadata().getName())
                        .addToLabels("public-keys", "true"))
                .build();
    }

    @Override
    public Optional<ConfigMap> distinguish(Class<ConfigMap> resource, Orbit primary, Context<Orbit> context) {
        return eventSource().orElseThrow().list(primary.getMetadata().getName()).findAny();
    }
}
