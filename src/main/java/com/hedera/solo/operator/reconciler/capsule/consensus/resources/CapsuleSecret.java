package com.hedera.solo.operator.reconciler.capsule.consensus.resources;

import static com.hedera.solo.operator.reconciler.common.Metadata.validName;
import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import io.fabric8.kubernetes.api.model.ObjectMeta;
import io.fabric8.kubernetes.api.model.Secret;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;

@KubernetesDependent
public class CapsuleSecret extends CRUDKubernetesDependentResource<Secret, ConsensusCapsule> {

    public static final String TEMPLATE = "secret.yaml";
    public static final String NAME_SUFFIX = "-keys";

    public CapsuleSecret() {
        super(Secret.class);
    }

    @Override
    protected Secret desired(ConsensusCapsule primary, Context<ConsensusCapsule> context) {
        Secret existing = context.getSecondaryResource(Secret.class).orElse(null);
        if (null != existing) {
            return existing;
        }
        ObjectMeta metadata = primary.getMetadata();
        String validName = validName(metadata.getName(), NAME_SUFFIX);

        return loadYaml(Secret.class, getClass(), TEMPLATE).edit()
                .editMetadata()
                .withName(validName)
                .withNamespace(metadata.getNamespace())
                .endMetadata()
                .build();
    }
}
