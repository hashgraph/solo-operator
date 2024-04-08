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

package com.hedera.solo.operator.reconciler.orbit.dependent;

import com.hedera.solo.operator.model.v1alpha1.Orbit;
import io.fabric8.kubernetes.api.model.Namespace;
import io.fabric8.kubernetes.api.model.NamespaceBuilder;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import jakarta.inject.Inject;
import org.jboss.logging.Logger;

@KubernetesDependent
public class OrbitNamespace extends CRUDKubernetesDependentResource<Namespace, Orbit> {

    @Inject
    Logger log;

    public OrbitNamespace() {
        super(Namespace.class);
    }

    @Override
    protected Namespace desired(final Orbit primary, final Context<Orbit> context) {
        final NamespaceBuilder builder = new NamespaceBuilder();

        builder.editOrNewSpec()
                .and()
                .editOrNewMetadata()
                .withName(primary.getMetadata().getName())
                .endMetadata();

        if (!primary.getMetadata().getLabels().isEmpty()) {
            builder.editOrNewMetadata()
                    .withLabels(primary.getMetadata().getLabels())
                    .endMetadata();
        }

        if (!primary.getMetadata().getAnnotations().isEmpty()) {
            builder.editOrNewMetadata()
                    .withAnnotations(primary.getMetadata().getAnnotations())
                    .endMetadata();
        }

        final Namespace namespace = builder.build();
        log.infof(
                "Creating namespace for Orbit [ orbit = '%s', namespace = '%s']",
                primary.getMetadata().getName(), namespace.getMetadata().getName());

        return namespace;
    }
}
