package com.hedera.solo.test.ni2;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;

import com.hedera.solo.operator.model.v1alpha1.NetworkInitialization;
import com.hedera.solo.operator.model.v1alpha1.NetworkInitializationSequence;
import io.fabric8.kubernetes.api.model.ObjectReference;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;

// if NIs are not cleaned up lifecycle (owner ref) should be tied to orbit instead.

@Deprecated
@KubernetesDependent
public class SeqDR extends CRUDKubernetesDependentResource<NetworkInitializationSequence, NetworkInitialization> {
    public SeqDR() {
        super(NetworkInitializationSequence.class);
    }

    @Override
    protected NetworkInitializationSequence desired(NetworkInitialization primary, Context<NetworkInitialization> context) {
        return edit(context.getSecondaryResource(NetworkInitializationSequence.class)
                .orElseGet(() -> edit(new NetworkInitializationSequence())
                        .with(builder -> builder
                                .editMetadata()
                                .withName(primary.getMetadata().getNamespace())
                                .withNamespace(primary.getMetadata().getNamespace()))
                .build()))
                .with(builder -> builder
                        .editSpec()
                        .addToSequence(edit(new ObjectReference())
                                .with(builder1 -> builder1
                                        .withName(primary.getMetadata().getName())
                                        .withUid(primary.getMetadata().getUid()))
                                .build()))
                .build();
    }
}
