/*
 * Copyright (C) 2024 Hedera Hashgraph, LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package com.hedera.solo.operator.reconciler.orbit.resources;

import com.hedera.solo.operator.model.v1alpha1.Orbit;
import com.hedera.solo.operator.reconciler.common.Metadata;
import io.fabric8.kubernetes.api.model.rbac.Role;
import io.fabric8.kubernetes.api.model.rbac.RoleBuilder;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ResourceDiscriminator;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDNoGCKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;

import java.util.Optional;

import static io.javaoperatorsdk.operator.ReconcilerUtils.loadYaml;

@KubernetesDependent(resourceDiscriminator = ConsensusCapsuleRole.Discriminator.class)
public class ConsensusCapsuleRole extends CRUDNoGCKubernetesDependentResource<Role, Orbit> {
    public static final String TEMPLATE = "consensus-capsule-role.yaml";
    public static final String NAME = "consensus-capsule-role";

    public ConsensusCapsuleRole() {
        super(Role.class);
    }

    @Override
    protected Role desired(final Orbit primary, final Context<Orbit> context) {
        final RoleBuilder role = loadYaml(Role.class, getClass(), TEMPLATE).edit();

        return Metadata.applyOrbitMetaTo(primary, role.editMetadata())
                .withNamespace(primary.getMetadata().getName())
                .endMetadata()
                .build();
    }

    public static class Discriminator implements ResourceDiscriminator<Role, Orbit> {
        @Override
        public Optional<Role> distinguish(
                final Class<Role> resource, final Orbit primary, final Context<Orbit> context) {
            return Optional.ofNullable(context.getClient().rbac().roles()
                    .inNamespace(primary.getMetadata().getName())
                    .withName(NAME)
                    .get());

        }
    }
}
