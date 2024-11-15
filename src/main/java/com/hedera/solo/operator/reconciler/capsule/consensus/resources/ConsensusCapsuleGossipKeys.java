package com.hedera.solo.operator.reconciler.capsule.consensus.resources;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;

import com.hedera.solo.operator.model.v1alpha1.ConsensusCapsule;
import com.hedera.solo.operator.reconciler.common.Metadata;
import io.fabric8.kubernetes.api.model.Secret;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ResourceDiscriminator;
import io.javaoperatorsdk.operator.processing.dependent.Creator;
import io.javaoperatorsdk.operator.processing.dependent.Updater;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResourceConfigBuilder;
import java.util.Map;
import java.util.Objects;
import java.util.Optional;
import java.util.Set;

@KubernetesDependent
public class ConsensusCapsuleGossipKeys extends KubernetesDependentResource<Secret, ConsensusCapsule> implements
        Creator<Secret, ConsensusCapsule>,
        ResourceDiscriminator<Secret, ConsensusCapsule>,
        Updater<Secret, ConsensusCapsule>
{
    public ConsensusCapsuleGossipKeys() {
        super(Secret.class);
        configureWith(new KubernetesDependentResourceConfigBuilder<Secret>()
                .withResourceDiscriminator(this)
                .build());
    }

    @Override
    public Optional<Secret> distinguish(Class<Secret> resource, ConsensusCapsule primary, Context<ConsensusCapsule> context) {
        return eventSource().orElseThrow().list(primary.getMetadata().getNamespace()).filter(x -> Objects.equals(name(primary), x.getMetadata().getName())).findAny();
    }

    @Override
    protected Secret desired(ConsensusCapsule primary, Context<ConsensusCapsule> context) {
        Secret secret = context.getSecondaryResource(Secret.class, this).orElseGet(() -> createSecret(primary));
        Set<String> keys = secret.getData().keySet();
        String name = "private-" + Metadata.prepareNodeName(primary) + ".pem";
        if (!(keys.contains("a-" + name) && keys.contains("s-" + name))) {
            secret = edit(secret)
                    .with(data -> data.addToData(Map.of(
                            "a-" + name, "aGVsbG8=",
                            "s-" + name, "aGVsbG8=")))
                    .build();
        }
        return secret;
//        keys.stream().anyMatch(k -> k.startsWith("a-private-" + primary.getMetadata().getName() + ".pem"));
//        if (secret.getData().containsKey())
//        return edit(context.getSecondaryResource(Secret.class, this).orElseGet(() -> createSecret(primary)))
//                .with(metadata -> metadata.editMetadata()
//                        .withName(name(primary))
//                        .withNamespace(primary.getMetadata().getNamespace()))
//                .build();
    }

    Secret createSecret(ConsensusCapsule primary) {
        return edit(new Secret())
                .with(metadata -> metadata.editMetadata()
                        .withName(name(primary))
                        .withNamespace(primary.getMetadata().getNamespace()))
                .build();
    }

    String name(ConsensusCapsule primary) {
        return Metadata.prepareNodeName(primary) + "-gossip-keys";
    }
}
