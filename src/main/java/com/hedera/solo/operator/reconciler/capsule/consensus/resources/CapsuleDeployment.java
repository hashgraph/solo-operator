package com.hedera.solo.operator.reconciler.capsule.consensus.resources;

import static com.hedera.solo.operator.reconciler.common.Metadata.validName;
import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import io.fabric8.kubernetes.api.model.ObjectMeta;
import io.fabric8.kubernetes.api.model.apps.Deployment;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;

@KubernetesDependent
public class CapsuleDeployment extends CRUDKubernetesDependentResource<Deployment, ConsensusCapsule> {

    public static final String TEMPLATE = "deployment.yaml";
    public static final String NAME_SUFFIX = "-deploy";

    public CapsuleDeployment() {
        super(Deployment.class);
    }

    @Override
    protected Deployment desired(ConsensusCapsule primary, Context<ConsensusCapsule> context) {
        ObjectMeta metadata = primary.getMetadata();
        String validName = validName(metadata.getName(), NAME_SUFFIX);

        boolean eligibleForDeployment = primary.getSpec() != null && primary.getSpec().getEligibleForDeployment();
        return loadYaml(Deployment.class, getClass(), TEMPLATE).edit()
                .editMetadata()
                .withName(validName)
                .withNamespace(metadata.getNamespace())
                .endMetadata()
                .editSpec()
                .withReplicas(eligibleForDeployment ? 1 : 0)
                .editTemplate()
                .editSpec()
                .editMatchingVolume(v -> "config-volume".equals(v.getName()))
                .editConfigMap()
                .withName(validName(metadata.getName(), "-cm"))
                .endConfigMap()
                .endVolume()
                .editMatchingVolume(v -> "secret-volume".equals(v.getName()))
                .editSecret()
                .withSecretName(validName(metadata.getName(), "-secret"))
                .endSecret()
                .endVolume()
                .endSpec()
                .endTemplate()
                .endSpec()
                .build();
    }
}
