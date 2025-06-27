// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.orbit.resources;

import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;

import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.reconciler.common.Constants;
import com.hedera.solo.operator.reconciler.common.Metadata;
import io.fabric8.kubernetes.api.model.Namespace;
import io.fabric8.kubernetes.api.model.NamespaceBuilder;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;

/**
 * The {@code OrbitNamespace} class represents a Kubernetes Namespace resource that is dependent on an Orbit custom
 * resource.
 */
@KubernetesDependent
public class OrbitNamespace extends CRUDKubernetesDependentResource<Namespace, Orbit> {
    public static final String TEMPLATE = "namespace.yaml";

    /** Constructs a new instance of {@code OrbitNamespace}. */
    public OrbitNamespace() {
        super(Namespace.class);
    }

    @Override
    protected Namespace desired(final Orbit primary, final Context<Orbit> context) {
        final String name = primary.getMetadata().getName();
        final String orbitName = primary.getMetadata().getName();
        final Namespace ns = loadYaml(Namespace.class, getClass(), TEMPLATE);
        final NamespaceBuilder nb =
                Metadata.applyOrbitMetaTo(primary, ns.edit().editMetadata()).endMetadata();
        return Metadata.applyStandardLabels(
                        nb.editMetadata(),
                        name,
                        orbitName,
                        Constants.COMPONENT_ORBIT,
                        Constants.COMPONENT_NAMESPACE,
                        null)
                .withName(name)
                .endMetadata()
                .build();
    }
}
