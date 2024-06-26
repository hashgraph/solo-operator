package com.hedera.solo.operator.reconciler.capsule.consensus.resources;

import static com.hedera.solo.operator.library.resource.SystemResource.getSystemResourceAsStringUnchecked;
import static com.hedera.solo.operator.reconciler.common.Metadata.validName;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsuleSpec;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.ConfigMapBuilder;
import io.fabric8.kubernetes.client.CustomResource;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import java.util.Optional;

@KubernetesDependent
public class CapsuleConfigMap extends CRUDKubernetesDependentResource<ConfigMap, ConsensusCapsule> {
    public static final String NAME_SUFFIX = "-cm";
    public CapsuleConfigMap() {
        super(ConfigMap.class);
    }

    @Override
    protected ConfigMap desired(ConsensusCapsule primary, Context<ConsensusCapsule> context) {
        return new ConfigMapBuilder()
                .withNewMetadata()
                .withName(validName(primary, NAME_SUFFIX))
                .withNamespace(primary.getMetadata().getNamespace())
                .endMetadata()
                .addToData("log4j2.xml", Optional.of(primary).map(CustomResource::getSpec).map(ConsensusCapsuleSpec::getLog4j2_xml).orElse(getSystemResourceAsStringUnchecked("hedera/mainnet/log4j2.xml")))
                .build();
    }
}
