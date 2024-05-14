package com.hedera.solo.operator.reconciler.capsule.consensus.resources;

import static com.hedera.solo.operator.reconciler.common.Metadata.validName;
import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import io.fabric8.kubernetes.api.model.ObjectMeta;
import io.fabric8.kubernetes.api.model.Service;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;

@KubernetesDependent
public class CapsuleService extends CRUDKubernetesDependentResource<Service, ConsensusCapsule> {

    public static final String TEMPLATE = "service.yaml";
    public static final String NAME_SUFFIX = "-svc";

    public CapsuleService() {
        super(Service.class);
    }

    @Override
    protected Service desired(ConsensusCapsule primary, Context<ConsensusCapsule> context) {
        ObjectMeta metadata = primary.getMetadata();
        String validName = validName(metadata.getName(), NAME_SUFFIX);

        return loadYaml(Service.class, getClass(), TEMPLATE).edit()
                .editMetadata()
                .withName(validName)
                .withNamespace(metadata.getNamespace())
                .endMetadata()
                .build();
    }
}
