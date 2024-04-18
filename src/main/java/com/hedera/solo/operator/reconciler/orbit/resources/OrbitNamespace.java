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
