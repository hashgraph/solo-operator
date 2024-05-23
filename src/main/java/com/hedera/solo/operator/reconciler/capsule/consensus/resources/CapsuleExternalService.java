package com.hedera.solo.operator.reconciler.capsule.consensus.resources;

import static com.hedera.solo.operator.reconciler.common.Metadata.validName;
import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import io.fabric8.kubernetes.api.model.ObjectMeta;
import io.fabric8.kubernetes.api.model.Service;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ResourceDiscriminator;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import java.util.Optional;

@KubernetesDependent(resourceDiscriminator = CapsuleExternalService.class)
public class CapsuleExternalService extends CRUDKubernetesDependentResource<Service, ConsensusCapsule> implements ResourceDiscriminator<Service, ConsensusCapsule> {

    public static final String TEMPLATE = "external-service.yaml";
    public static final String NAME_SUFFIX = "-svc";

    public CapsuleExternalService() {
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
                .editSpec()
                .addToSelector("name", metadata.getName())
                .endSpec()
                .build();
    }

    @Override
    public Optional<Service> distinguish(Class<Service> resource, ConsensusCapsule primary, Context<ConsensusCapsule> context) {
        return context.getSecondaryResources(Service.class).stream().filter(s -> s.getMetadata().getName().endsWith(NAME_SUFFIX)).findFirst();
    }
}
