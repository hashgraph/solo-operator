// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.orbit.resources;

import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;

import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.reconciler.common.Metadata;
import io.fabric8.kubernetes.api.model.Namespace;
import io.fabric8.kubernetes.api.model.NamespaceBuilder;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;

@KubernetesDependent
public class OrbitNamespace extends CRUDKubernetesDependentResource<Namespace, Orbit> {
    public static final String TEMPLATE = "namespace.yaml";

    public OrbitNamespace() {
        super(Namespace.class);
    }

    @Override
    protected Namespace desired(final Orbit primary, final Context<Orbit> context) {
        final NamespaceBuilder builder =
                loadYaml(Namespace.class, getClass(), TEMPLATE).edit();

        return Metadata.applyOrbitMetaTo(primary, builder.editMetadata())
                .withName(primary.getMetadata().getName())
                .endMetadata()
                .build();
    }
}
