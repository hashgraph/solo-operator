package com.hedera.solo.test.labelselector;

import static com.hedera.solo.operator.library.builder.BuilderWrapper.edit;

import com.hedera.solo.operator.test.v1alpha1.Example;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;

@KubernetesDependent
public class NormalResource extends CRUDKubernetesDependentResource<ConfigMap, Example> {
    public NormalResource() {
        super(ConfigMap.class);
    }

    @Override
    protected ConfigMap desired(Example primary, Context<Example> context) {
        return edit(new ConfigMap())
                .with(builder -> builder
                        .editMetadata()
                        .withName("normal")
                        .withLabels(primary.getMetadata().getLabels()))
                .build();
    }
}
