// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.test.example;

import com.hedera.solo.operator.test.v1alpha1.Example;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;

@KubernetesDependent
public class ExampleConfigMap extends CRUDKubernetesDependentResource<ConfigMap, Example> {
    public ExampleConfigMap() {
        super(ConfigMap.class);
    }

    @Override
    protected ConfigMap desired(Example primary, Context<Example> context) {
        return context.getSecondaryResource(ConfigMap.class)
                .orElseGet(ConfigMap::new)
                .edit()
                .editMetadata()
                .withName(primary.getMetadata().getName())
                .withNamespace(primary.getMetadata().getNamespace())
                .endMetadata()
                .addToData("message", primary.getSpec().getMessage())
                .build();
    }
}
