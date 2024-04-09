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
import io.fabric8.kubernetes.api.model.ConfigMap;
import io.fabric8.kubernetes.api.model.ConfigMapBuilder;
import io.javaoperatorsdk.operator.api.reconciler.Context;
import io.javaoperatorsdk.operator.api.reconciler.ResourceDiscriminator;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.CRUDNoGCKubernetesDependentResource;
import io.javaoperatorsdk.operator.processing.dependent.kubernetes.KubernetesDependent;
import java.util.Optional;

@KubernetesDependent(resourceDiscriminator = ConsensusConfigMap.Discriminator.class)
public class ConsensusConfigMap extends CRUDNoGCKubernetesDependentResource<ConfigMap, Orbit> {

    public static final String TEMPLATE = "consensus-config-cm.yaml";
    public static final String NAME = "consensus-config";

    public ConsensusConfigMap() {
        super(ConfigMap.class);
    }

    @Override
    protected ConfigMap desired(final Orbit primary, final Context<Orbit> context) {
        final ConfigMapBuilder cm =
                loadYaml(ConfigMap.class, getClass(), TEMPLATE).edit();

        return Metadata.applyOrbitMetaTo(primary, cm.editMetadata())
                .withNamespace(primary.getMetadata().getName())
                .endMetadata()
                .build();
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
