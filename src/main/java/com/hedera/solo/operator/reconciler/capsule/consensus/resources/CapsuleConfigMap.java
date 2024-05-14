package com.hedera.solo.operator.reconciler.capsule.consensus.resources;

import static com.hedera.solo.operator.reconciler.common.Metadata.validName;
import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.ObjectMeta;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;

@KubernetesDependent
public class CapsuleConfigMap extends CRUDKubernetesDependentResource<ConfigMap, ConsensusCapsule> {

    public static final String TEMPLATE = "config-map.yaml";
    public static final String NAME_SUFFIX = "-cm";

    public CapsuleConfigMap() {
        super(ConfigMap.class);
    }

    @Override
    protected ConfigMap desired(ConsensusCapsule primary, Context<ConsensusCapsule> context) {
        ObjectMeta metadata = primary.getMetadata();
        String validName = validName(metadata.getName(), NAME_SUFFIX);

        return loadYaml(ConfigMap.class, getClass(), TEMPLATE).edit()
                .editMetadata()
                .withName(validName)
                .withNamespace(metadata.getNamespace())
                .endMetadata()
                .build();
    }
}
