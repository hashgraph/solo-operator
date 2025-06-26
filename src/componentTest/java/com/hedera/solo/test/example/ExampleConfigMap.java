// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.test.example;

import com.hedera.solo.operator.test.v1alpha1.Example;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;

/**
 * The ExampleConfigMap class is a Kubernetes dependent resource that manages ConfigMap resources for the Example
 * operator.
 */
@KubernetesDependent
public class ExampleConfigMap extends CRUDKubernetesDependentResource<ConfigMap, Example> {
    /** Constructs an instance of ExampleConfigMap with the desired type set to ConfigMap. */
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
