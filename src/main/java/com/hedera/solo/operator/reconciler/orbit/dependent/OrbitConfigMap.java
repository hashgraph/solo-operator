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
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.ConfigMapBuilder;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ResourceDiscriminator;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDNoGCKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import jakarta.inject.Inject;
import java.util.Optional;
import org.jboss.logging.Logger;

@KubernetesDependent(resourceDiscriminator = OrbitConfigMap.Discriminator.class)
public class OrbitConfigMap extends CRUDNoGCKubernetesDependentResource<ConfigMap, Orbit> {

    public static final String NAME = "orbit-config";

    @Inject
    Logger log;

    public OrbitConfigMap() {
        super(ConfigMap.class);
    }

    @Override
    protected ConfigMap desired(final Orbit primary, final Context<Orbit> context) {
        final ConfigMapBuilder builder = new ConfigMapBuilder();
        builder.editOrNewMetadata()
                .withName(NAME)
                .withNamespace(primary.getMetadata().getName())
                .endMetadata();

        if (!primary.getMetadata().getLabels().isEmpty()) {
            builder.editOrNewMetadata()
                    .withLabels(primary.getMetadata().getLabels())
                    .endMetadata();
        }

        builder.editOrNewMetadata()
                .addToLabels(
                        "operator.solo.hedera.com/orbit", primary.getMetadata().getName())
                .endMetadata();

        if (!primary.getMetadata().getAnnotations().isEmpty()) {
            builder.editOrNewMetadata()
                    .withAnnotations(primary.getMetadata().getAnnotations())
                    .endMetadata();
        }

        return builder.build();
    }

    public static class Discriminator implements ResourceDiscriminator<ConfigMap, Orbit> {
        @Override
        public Optional<ConfigMap> distinguish(
                final Class<ConfigMap> resource, final Orbit primary, final Context<Orbit> context) {
            return Optional.ofNullable(context.getClient()
                    .configMaps()
                    .inNamespace(primary.getMetadata().getName())
                    .withName(NAME)
                    .get());
        }
    }
}
