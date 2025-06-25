// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.orbit.resources;

import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;

import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.reconciler.common.Constants;
import com.hedera.solo.operator.reconciler.common.Metadata;
import io.fabric8.kubernetes.api.model.rbac.Role;
import io.fabric8.kubernetes.api.model.rbac.RoleBuilder;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDNoGCKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;

/**
 * The {@code ConsensusCapsuleRole} class is a Kubernetes dependent resource that manages the creation and update of a
 * Role for consensus capsules.
 */
@KubernetesDependent
public class ConsensusCapsuleRole extends CRUDNoGCKubernetesDependentResource<Role, Orbit> {
    public static final String TEMPLATE = "consensus-capsule-role.yaml";
    public static final String NAME = "consensus-capsule-role";

    /** Default constructor for {@code ConsensusCapsuleRole}. */
    public ConsensusCapsuleRole() {
        super(Role.class);
    }

    @Override
    protected Role desired(final Orbit primary, final Context<Orbit> context) {
        final String orbitName = primary.getMetadata().getName();
        final Role role = loadYaml(Role.class, getClass(), TEMPLATE);
        final RoleBuilder rb =
                Metadata.applyOrbitMetaTo(primary, role.edit().editMetadata()).endMetadata();
        return Metadata.applyStandardLabels(
                        rb.editMetadata(), NAME, orbitName, Constants.COMPONENT_ORBIT, Constants.COMPONENT_ROLE, null)
                .withNamespace(primary.getMetadata().getName())
                .endMetadata()
                .build();
    }
}
