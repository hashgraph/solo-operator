package com.hedera.solo.operator.reconciler.capsule.consensus.resources;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import io.fabric8.kubernetes.api.model.Secret;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ResourceDiscriminator;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResourceConfigBuilder;
import java.util.Objects;
import java.util.Optional;

@KubernetesDependent
public class ConsensusCapsuleGrpcTlsKeys extends CRUDKubernetesDependentResource<Secret, ConsensusCapsule> implements ResourceDiscriminator<Secret, ConsensusCapsule> {
    public ConsensusCapsuleGrpcTlsKeys() {
        super(Secret.class);
        configureWith(new KubernetesDependentResourceConfigBuilder<Secret>()
                .withResourceDiscriminator(this)
                .build());
    }

    @Override
    public Optional<Secret> distinguish(Class<Secret> resource, ConsensusCapsule primary, Context<ConsensusCapsule> context) {
        String name = primary.getMetadata().getName() + "node0-grpc-tls-keys";
        return eventSource().orElseThrow().list(primary.getMetadata().getNamespace()).filter(x -> Objects.equals(name, x.getMetadata().getName())).findAny();
    }

    @Override
    protected Secret desired(ConsensusCapsule primary, Context<ConsensusCapsule> context) {
        String name = primary.getMetadata().getName() + "node0-grpc-tls-keys";
        return edit(new Secret())
                .with(metadata -> metadata.editMetadata()
                        .withName(name)
                        .withNamespace(primary.getMetadata().getNamespace()))
                .build();
    }
}
