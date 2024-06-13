package com.hedera.solo.operator.reconciler.capsule.consensus.resources;

import com.hedera.solo.operator.library.dependentresources.ManyToOneKubernetesDependentResource;
import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.ConfigMapBuilder;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;

@KubernetesDependent
public class CapsuleRegistry
        extends ManyToOneKubernetesDependentResource<ConfigMap, ConsensusCapsule> {
    public static final String NAME = "consensus-capsules";

    public CapsuleRegistry() {
        super(ConfigMap.class);
    }

    @Override
    protected ConfigMap create(ConsensusCapsule primary) {
        return new ConfigMapBuilder()
                .editMetadata()
                .withNamespace(primary.getMetadata().getNamespace())
                .withName(NAME)
                .endMetadata()
                .build();
    }
}
