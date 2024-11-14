// SPDX-License-Identifier: Apache-2.0

package com.hedera.solo.operator.reconciler.orbit.resources;

import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;

import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.reconciler.common.Metadata;
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.rbac.Role;
import io.fabric8.kubernetes.api.model.rbac.RoleBuilder;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ResourceDiscriminator;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDNoGCKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependentResourceConfigBuilder;
import java.util.Optional;

@KubernetesDependent
public class ConsensusCapsuleRole extends CRUDNoGCKubernetesDependentResource<Role, Orbit> {
    public static final String TEMPLATE = "consensus-capsule-role.yaml";
    public static final String NAME = "consensus-capsule-role";

    public ConsensusCapsuleRole() {
        super(Role.class);
        configureWith(new KubernetesDependentResourceConfigBuilder<Role>().build());
    }

    @Override
    protected Role desired(final Orbit primary, final Context<Orbit> context) {
        final RoleBuilder role = loadYaml(Role.class, getClass(), TEMPLATE).edit();

        return Metadata.applyOrbitMetaTo(primary, role.editMetadata())
                .withNamespace(primary.getMetadata().getName())
                .endMetadata()
                .build();
    }
}
